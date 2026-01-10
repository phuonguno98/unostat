# Cơ Chế Thu Thập Metrics

Tài liệu này mô tả chi tiết cách thức **UnoStat** thu thập, tính toán và xử lý các chỉ số hiệu năng hệ thống (CPU, RAM, Disk, Network).

## 1. Nguyên Lý Chung

UnoStat hoạt động dựa trên phương pháp **Delta Sampling** (Lấy mẫu dựa trên sự thay đổi). Vì hầu hết các bộ đếm (counter) của hệ thống (như `/proc/stat` trên Linux) là các giá trị tăng dần (monotonic counters), UnoStat không sử dụng giá trị tức thời (ngoại trừ RAM) mà tính toán hiệu năng trong một khoảng thời gian $\Delta t$.

**Quy trình thu thập:**
1.  **Lần chạy đầu tiên ($t_0$):** Đọc các giá trị bộ đếm thô từ hệ thống và lưu vào bộ nhớ cache (`prevStats`). Ở bước này chưa có metrics nào được xuất ra.
2.  **Các lần chạy tiếp theo ($t_1, t_2, \dots$):**
    *   Đọc giá trị bộ đếm hiện tại (`currentStats`).
    *   Tính độ chênh lệch: $\Delta = currentStats - prevStats$.
    *   Áp dụng công thức tính toán để ra metric cuối cùng.
    *   Cập nhật `prevStats = currentStats`.

## 2. Chi Tiết Các Metrics

### 2.1. CPU

*   **Thư viện:** `github.com/shirou/gopsutil/v3/cpu`
*   **Hàm gọi:** `Times(false)` (Aggregated - Tổng hợp tất cả các core).

#### A. CPU Utilization (Độ bận CPU)
Đo lường phần trăm thời gian CPU đang xử lý công việc (không rảnh).

$$
\text{TotalTime} = \text{User} + \text{System} + \text{Idle} + \text{IOWait} + \text{Irq} + \text{SoftIrq} + \text{Steal} + \text{Guest}
$$

$$
\Delta\text{Total} = \text{TotalTime}_{curr} - \text{TotalTime}_{prev}
$$
$$
\Delta\text{Idle} = \text{Idle}_{curr} - \text{Idle}_{prev}
$$

$$
\text{CPU \%} = 100 \times \left(1 - \frac{\Delta\text{Idle}}{\Delta\text{Total}}\right)
$$

#### B. CPU IOWait
Đo lường phần trăm thời gian CPU nhàn rỗi nhưng đang chờ I/O đĩa hoàn tất. Đây là chỉ số quan trọng để phát hiện nghẽn cổ chai đĩa.

$$
\Delta\text{IOWait} = \text{IOWait}_{curr} - \text{IOWait}_{prev}
$$

$$
\text{IOWait \%} = 100 \times \left(\frac{\Delta\text{IOWait}}{\Delta\text{Total}}\right)
$$

**Lưu ý về đa nền tảng:**
*   **Linux:** Hỗ trợ đầy đủ và chính xác.
*   **Windows:** Không hỗ trợ (luôn trả về `-1.0`).
*   **macOS:** Hỗ trợ hạn chế (có thể trả về `-1.0` nếu không lấy được).

### 2.2. Memory (RAM)

*   **Thư viện:** `github.com/shirou/gopsutil/v3/mem`
*   **Hàm gọi:** `VirtualMemory()`

Khác với CPU hay Disk, RAM là trạng thái tức thời (Instantaneous State), không cần tính Delta.

$$
\text{RAM \%} = \left(\frac{\text{Used}}{\text{Total}}\right) \times 100
$$

### 2.3. Disk I/O

*   **Thư viện:** `github.com/shirou/gopsutil/v3/disk`
*   **Hàm gọi:** `IOCounters()`
*   **Lọc thiết bị:** Hỗ trợ whitelist/blacklist tên thiết bị để tránh các loop device hoặc RAM disk.

#### A. Disk Utilization (Độ bận đĩa)
Đo lường phần trăm thời gian đĩa có ít nhất một request I/O đang được xử lý.

Do `gopsutil` trả về `IoTime` (tổng thời gian đĩa bận tích lũy) tính bằng mili-giây:

$$
\Delta\text{Time} = \text{Time}_{curr} - \text{Time}_{prev} \quad (\text{ms})
$$
$$
\Delta\text{IoTime} = \text{IoTime}_{curr} - \text{IoTime}_{prev}
$$

$$
\text{Disk Util \%} = \min\left(100, \frac{\Delta\text{IoTime}}{\Delta\text{Time}} \times 100\right)
$$

**Xử lý đặc biệt cho Windows:**
Trên Windows, `IoTime` thường không có sẵn, tool sẽ sử dụng xấp xỉ: `IoTime ≈ ReadTime + WriteTime`.

#### B. Disk Await (Độ trễ trung bình)
Thời gian trung bình (ms) để hoàn thành một I/O request (bao gồm cả thời gian đợi trong queue và thời gian xử lý vật lý).

$$
\Delta\text{Ops} = (\text{ReadCount} + \text{WriteCount})_{curr} - (\text{ReadCount} + \text{WriteCount})_{prev}
$$
$$
\Delta\text{IOTimeTotals} = (\text{ReadTime} + \text{WriteTime})_{curr} - (\text{ReadTime} + \text{WriteTime})_{prev}
$$

$$
\text{Await (ms)} = \begin{cases}
0 & \text{if } \Delta\text{Ops} = 0 \\
\frac{\Delta\text{IOTimeTotals}}{\Delta\text{Ops}} & \text{if } \Delta\text{Ops} > 0
\end{cases}
$$

#### C. Disk IOPS (Throughput)
Số lượng thao tác đọc và ghi đĩa được thực hiện mỗi giây.

$$
\text{IOPS} = \frac{\Delta\text{Ops}}{\Delta\text{Time (seconds)}}
$$

### 2.4. Network Bandwidth

*   **Thư viện:** `github.com/shirou/gopsutil/v3/net`
*   **Hàm gọi:** `IOCounters(true)` (Lấy chi tiết từng interface).
*   **Lọc:** Tự động loại bỏ Loopback interfaces (như `lo`, `127.0.0.1`).

Tính toán tổng băng thông (Upload + Download):

$$
\Delta\text{Bytes} = (\text{BytesSent} + \text{BytesRecv})_{curr} - (\text{BytesSent} + \text{BytesRecv})_{prev}
$$

$$
\text{Bandwidth (bps)} = \frac{\Delta\text{Bytes} \times 8}{\Delta\text{Time (seconds)}}
$$

*Kết quả sau đó được chuyển đổi sang Kbps hoặc Mbps để hiển thị.*

## 3. Tổng Kết Luồng Dữ Liệu

1.  **Collector Manager** khởi chạy các collectors riêng lẻ (CPU, MEM, Disk, Network).
2.  Mỗi collector thực hiện snapshot dữ liệu hệ thống.
3.  So sánh với snapshot trước đó để tính metrics.
4.  Lưu snapshot hiện tại làm mốc cho lần sau.
5.  Dữ liệu metrics được tổng hợp và gửi đi (qua CSV Writer hoặc API Server).
