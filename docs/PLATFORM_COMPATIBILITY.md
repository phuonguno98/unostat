# Platform Compatibility Matrix - UnoStat

---

## 1. Tổng Quan

Tài liệu này xác định mức độ tương thích và khả năng hỗ trợ của UnoStat trên các hệ điều hành phổ biến. Do sự khác biệt về kiến trúc Kernel, một số chỉ số (metrics) sẽ có độ tin cậy hoặc phương pháp thu thập khác nhau tùy nền tảng.

---

## 2. Metrics Availability Matrix (Bảng Khả Dụng)

| Metric | Linux | Windows | macOS | Ghi chú |
|--------|:-----:|:-------:|:-----:|---------|
| **CPU Utilization** | ✅ | ✅ | ✅ | Hỗ trợ đầy đủ trên mọi nền tảng. |
| **CPU IO Wait** | ✅ | ❌ | ⚠️ | Windows luôn trả về `-1.0`. macOS thường không chính xác (-1.0). |
| **RAM Utilization** | ✅ | ✅ | ✅ | Tính toán dựa trên Virtual Memory Global Status. |
| **Disk Utilization** | ✅ | ✅* | ⚠️ | (*) Windows dùng xấp xỉ `(ReadTime+WriteTime)`. macOS yêu cầu quyền Full Disk Access. |
| **Disk Await** | ✅ | ✅ | ⚠️ | Độ trễ trung bình mỗi thao tác I/O. |
| **Disk IOPS** | ✅ | ✅ | ⚠️ | Số lượng thao tác đọc/ghi mỗi giây. |
| **Network Monitor** | ✅ | ✅ | ✅ | Tự động loại bỏ Loopback interfaces. |

**Chú giải:**
- ✅ : **Supported** (Hoạt động chính xác, ổn định).
- ⚠️ : **Limited** (Có hạn chế về độ chính xác hoặc yêu cầu quyền hạn đặc biệt).
- ❌ : **Not Supported** (Không khả dụng do hạn chế của OS API).

---

## 3. Chi Tiết Nền Tảng

### 3.1. Linux (Primary Target)
Đây là môi trường hoạt động tốt nhất của UnoStat.
*   **Source:** Sử dụng trực tiếp `/proc/stat`, `/proc/diskstats`, `/proc/net/dev`.
*   **Container Support:** Hiện tại UnoStat đọc metrics từ các file `/proc` của hệ thống. Khi chạy trong Docker/Container:
    *   Metrics CPU/Disk/Net sẽ phản ánh thông số của **Host Kernel** (trừ khi `/proc` được mount riêng biệt).
    *   Chưa hỗ trợ Native Cgroup Metrics (sẽ có trong các phiên bản sau).

### 3.2. Windows
*   **CPU IO Wait:** Khái niệm `iowait` (thời gian CPU rảnh chờ đĩa) không tồn tại trực tiếp trong bộ đếm hiệu năng của Windows theo cách giống Linux. UnoStat sẽ luôn trả về giá trị `-1.0` để biểu thị sự không khả dụng.
*   **Disk Utilization:** Bộ đếm `IoTime` (thời gian đĩa bận) thường không có sẵn hoặc bằng 0 trên nhiều phiên bản Windows (trừ khi bật PerfCounters đặc biệt).
    *   **Cơ chế Fallback:** Nếu `IoTime == 0`, UnoStat sẽ ước lượng bằng tổng `ReadTime + WriteTime`.

### 3.3. macOS (Darwin)
*   **Permissions:** Để đọc được thông số Disk I/O chi tiết, ứng dụng thường yêu cầu quyền root (`sudo`) hoặc cấp quyền **"Full Disk Access"** trong System Settings.
*   **IO Wait:** Tương tự Windows, chỉ số này thường không tin cậy trên macOS và thường trả về `-1.0`.

---

## 4. Kiểm Thử & Triển Khai

### 4.1. Kiến Trúc CPU
UnoStat hỗ trợ build native (không CGO bắt buộc) cho:
*   **amd64 (x86_64):** Intel/AMD CPUs.
*   **arm64:** Apple Silicon (M1/M2/...) và các server ARM (AWS Graviton).

### 4.2. Khuyến Nghị Triển Khai
Để đảm bảo ổn định khi chạy lâu dài (Load Testing):
1.  **Linux:** Chạy dưới dạng Systemd Service.
2.  **Windows:** Sử dụng `NSSM` để cài đặt `.exe` thành Windows Service, hoặc chạy trong PowerShell session với quyền Administrator để đảm bảo truy cập được toàn bộ Performance Counters.
