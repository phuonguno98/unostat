# UnoStat - Lightweight System Performance Monitoring for Performance Testing

[![CI Status](https://github.com/phuonguno98/unostat/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/phuonguno98/unostat/actions/workflows/ci-cd.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/phuonguno98/unostat)](https://go.dev/)
[![Latest Release](https://img.shields.io/github/v/release/phuonguno98/unostat)](https://github.com/phuonguno98/unostat/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**UnoStat** là công cụ giám sát hiệu năng hệ thống gọn nhẹ, đa nền tảng được viết bằng Go. Chuyên biệt cho **Kiểm thử hiệu năng (Performance Testing)** để theo dõi thời gian thực CPU, RAM, Disk Utilization (busy time), Await và Băng thông mạng (Network Bandwidth) với khả năng xuất dữ liệu CSV.

Ứng dụng tập trung vào việc thu thập, lưu trữ và trực quan hóa các chỉ số tài nguyên quan trọng (CPU, RAM, Disk, Network) theo thời gian thực với độ chính xác cao, giúp quản trị viên hệ thống và tester dễ dàng phân tích điểm nghẽn (bottleneck) của hệ thống dưới áp lực tải, cung cấp thông tin tải hệ thống cho báo cáo.

---

## 🚀 Tính năng nổi bật

*   **Giám sát thời gian thực:** Thu thập chỉ số với độ trễ thấp và tần suất tùy chỉnh (ví dụ: 1s, 5s, 30s).
*   **Rotation Logic:** Tự động cắt file kết quả ra file mới nếu dung lượng vượt quá **150MB** để dễ dàng quản lý (vd: `data.csv` -> `data_1.csv`).
*   **Chỉ số chuyên sâu cho Performance Testing:**
    *   **CPU:** Utilization (User/System/Idle) và iowait (phát hiện nghẽn I/O).
    *   **Disk:** Utilization (Busy Time %), Await (độ trễ phản hồi trung bình) và IOPS.
    *   **Network:** Total Bandwidth (bits/s) - Tổng băng thông In + Out.
    *   **RAM:** Utilization (%).
*   **Xuất dữ liệu CSV:** Tự động lưu dữ liệu thô ra file CSV để phân tích sau hoặc import vào **UnoStat Dashboard** và các công cụ khác (JMeter, Excel).
*   **Giao diện UnoStat Dashboard trực quan:**
    *   Server dựng sẵn để upload và xem biểu đồ từ file CSV.
    *   Hỗ trợ lọc theo khoảng thời gian (Time Range Filter).
    *   Xuất biểu đồ ra hình ảnh chất lượng cao phục vụ báo cáo.
*   **Kiến trúc gọn nhẹ:** Viết bằng Go, biên dịch ra binary duy nhất, không cần cài đặt dependencies phức tạp.
*   **Đa nền tảng:** Chạy tốt trên Windows, Linux và macOS.

---

## 🛠 Công nghệ sử dụng

*   **Core:** [![Go Version](https://img.shields.io/github/go-mod/go-version/phuonguno98/unostat)](https://go.dev/) - Hiệu năng cao, concurrency mạnh mẽ.
*   **System Info:** `github.com/shirou/gopsutil` - Thư viện chuẩn để lấy thông tin hệ thống đa nền tảng.
*   **Web Server:** `github.com/gorilla/mux` - High performance HTTP router.
*   **CLI:** `github.com/spf13/cobra` - Xây dựng giao diện dòng lệnh chuyên nghiệp.
*   **Frontend:** HTML5, CSS3, Vanilla JS (không framework nặng nề) để hiển thị biểu đồ nhanh chóng.

---

## 📊 Phương thức thu thập dữ liệu (Metrics Collection)

> **Tài liệu kỹ thuật:** Xem chi tiết các công thức toán học và cơ chế tính toán tại [Metrics Collection Docs](docs/METRICS_COLLECTION.md).

UnoStat sử dụng phương pháp **lấy mẫu delta (Delta Sampling)** để đảm bảo độ chính xác thay vì chỉ lấy giá trị tức thời.

### 1. CPU Utilization
**Phương thức:** Đọc bộ đếm thời gian của CPU (`/proc/stat` trên Linux) tại thời điểm T1 và T2.

**Công thức:**

$$
\text{Utilization \%} = 100 - \frac{(\text{Idle}_{T2} - \text{Idle}_{T1})}{(\text{Total}_{T2} - \text{Total}_{T1})} \times 100
$$

**Ý nghĩa:** Phản ánh chính xác phần trăm thời gian CPU đang bận xử lý công việc trong khoảng thời gian lấy mẫu.

### 2. Memory (RAM)
**Phương thức:** Đọc thông tin bộ nhớ ảo từ hệ điều hành. RAM là trạng thái tức thời (Instantaneous State), không cần tính Delta.

**Công thức:**

$$
\text{RAM \%} = \left(\frac{\text{Used}}{\text{Total}}\right) \times 100
$$

### 3. Disk Utilization & Await
**Phương thức:** Đọc `/proc/diskstats` (Linux) hoặc Performance Counters (Windows).

**Utilization:** Tính toán dựa trên `IoTime` (thời gian ổ cứng bận rộn).

$$
\text{Utils \%} = \frac{(\text{IoTime}_{T2} - \text{IoTime}_{T1})}{\Delta T} \times 100
$$

**Await:** Độ trễ trung bình của một request I/O.

$$
\text{Await (ms)} = \frac{(\text{ReadTime} + \text{WriteTime})_{delta}}{(\text{ReadCount} + \text{WriteCount})_{delta}}
$$

**IOPS:** Số lượng thao tác đọc/ghi trên ổ cứng mỗi giây.

$$
\text{IOPS} = \frac{(\text{ReadCount} + \text{WriteCount})_{delta}}{\Delta T}
$$

### 4. Network Bandwidth
**Phương thức:** Tính tổng chênh lệch `BytesSent` và `BytesRecv` giữa hai lần lấy mẫu.

**Công thức:**

$$
\text{Bandwidth} = \frac{(\text{BytesSent} + \text{BytesRecv})_{delta} \times 8}{\Delta T} \text{ (bits/s)}
$$

---

## 📖 Hướng dẫn sử dụng

> **Tài liệu chi tiết:** Để xem hướng dẫn đầy đủ về mọi tùy chọn và câu lệnh CLI, vui lòng xem [UnoStat CLI User Guide](docs/USER_GUIDE.md).

Quy trình sử dụng gồm 3 bước chính: **Cài đặt** -> **Thu thập dữ liệu** (trên máy cần test) -> **Phân tích** (trên máy quản trị).

### 1. Cài đặt & Build

Yêu cầu: Đã cài đặt [Go 1.25+](https://go.dev/dl/).

Trước tiên, hãy clone mã nguồn về máy:
```bash
git clone https://github.com/phuonguno98/unostat.git
cd unostat
```

**Cách 1: Sử dụng Makefile (Khuyên dùng)**
```bash
# Build trọn bộ vào thư mục bin/
make build
```

**Cách 2: Build thủ công**
```bash
# Windows
go build -o bin/unostat.exe ./cmd/unostat

# Linux/macOS
go build -o bin/unostat ./cmd/unostat
```

### 2. Thu thập dữ liệu (Collector)

Chạy tool `unostat` trực tiếp trên máy chủ (Server) đang chịu tải để ghi lại metrics.

**Kiểm tra thiết bị trước khi chạy:**
```bash
# Liệt kê tên các ổ đĩa và card mạng mà tool nhận diện được
./bin/unostat list-devices
```

**Các kịch bản thu thập phổ biến:**

*   **Kịch bản A: Chạy cơ bản (Auto Mode)**
    Thu thập tất cả, mặc định 30s/lần, lưu file tại chỗ.
    ```bash
    ./bin/unostat collect
    ```

*   **Kịch bản B: Khớp với chu kỳ Load Test**
    Nếu kịch bản test của bạn cần độ mịn cao (ví dụ Ramp-up nhanh), hãy giảm `interval`.
    ```bash
    # Lấy mẫu 5 giây/lần, xuất ra file riêng
    ./bin/unostat collect --interval 5s --output ./report/loadtest_result.csv
    ```

*   **Kịch bản C: Lọc nhiễu (Production Mode)**
    Chỉ giám sát ổ Data và Card mạng thực tế, bỏ qua ổ hệ thống hoặc Loopback.
    ```bash
    # Windows: Chỉ theo dõi ổ D, bỏ qua card Loopback
    ./bin/unostat collect --include-disks "D:" --exclude-networks "Loopback Pseudo-Interface 1"

    # Linux: Chỉ theo dõi sdb, eth0
    ./bin/unostat collect --include-disks "sdb" --include-networks "eth0"
    ```



### 3. Phân tích báo cáo (Visualizer)

Sau khi có file CSV từ bước 2, sử dụng lệnh `visualize` để xem biểu đồ trực quan.

**Khởi chạy Dashboard:**
```bash
# Chạy dashboard ở port 3000 và tự động mở trình duyệt
./bin/unostat visualize --port 3000 --open-browser
```

**Thao tác trên giao diện Web (http://127.0.0.1:3000):**
1.  Nhấn nút **Upload CSV** (góc trái) và chọn file kết quả `.csv`.
2.  Hệ thống sẽ vẽ các biểu đồ tương ứng: **CPU**, **Memory**, **Disk I/O** (Util/Await), **Network**.
3.  **Zoom:** Kéo chuột trái chọn vùng trên biểu đồ để phóng to khoảng thời gian xuất hiện lỗi (Spike).
4.  **Reset Zoom:** Nhấp đúp chuột vào biểu đồ để về mặc định.

---

## ⚙️ Giới hạn & Cấu hình mặc định

### 1. Client (unostat) - Thu thập dữ liệu

| Tham số | Cờ (Flag) | Giá trị mặc định | Mô tả chi tiết |
| :--- | :--- | :--- | :--- |
| **Interval** | `--interval` | `30s` | Khoảng cách giữa các lần lấy mẫu. Hỗ trợ định dạng `1s`, `1m`, `1h`. |
| **Output** | `--output` | `<hostname>_<timestamp>.csv` | File kết quả. Mặc định tạo file mới với tên theo timestamp tại thư mục hiện tại. |
| **Buffer Size** | `--buffer-size` | `100` | Số lượng dòng metric lưu trong RAM trước khi ghi xuống đĩa cứng. Giúp giảm I/O và tránh xung đột với chính disk cần giám sát. |
| **Flush Interval** | `--flush-interval` | `5s` | Thời gian tối đa giữ dữ liệu trong bộ nhớ đệm. Nếu chưa đầy Buffer Size nhưng đã quá thời gian này, dữ liệu vẫn sẽ được ghi. |
| **Log Level** | `--log-level` | `info` | Mức độ log (`debug`, `info`, `warn`, `error`). |
| **Timezone** | `--timezone` | `Local` | Múi giờ ghi trong cột Timestamp của CSV (VD: `Asia/Ho_Chi_Minh`). Nếu không set sẽ dùng giờ hệ thống máy chạy. |
| **Include/Exclude** | `--include-disks`<br>`--exclude-networks` | `""` (Rỗng) | Mặc định giám sát **tất cả** thiết bị tìm thấy. Dùng dấu phẩy `,` để phân cách nhiều thiết bị. |
| **File Rotation** | (Auto) | `150 MB` | Cơ chế tự động cắt file khi dung lượng vượt quá giới hạn. File mới sẽ có suffix `_N` (VD: `_1.csv`). |

### 2. Server (UnoStat Dashboard) - Giao diện Web

| Tham số | Cờ (Flag) | Giá trị mặc định | Mô tả chi tiết |
| :--- | :--- | :--- | :--- |
| **Host** | `--host` | `0.0.0.0` | Địa chỉ IP để lắng nghe (Listen Address). Mặc định `0.0.0.0` (tất cả interfaces). Set `127.0.0.1` nếu chỉ muốn truy cập local. |
| **Port** | `--port`, `-p` | `8080` | Cổng HTTP cho giao diện Web. Truy cập qua `http://localhost:<port>`. |
| **Upload Directory** | `--upload-dir`, `-d` | `./uploads` | Thư mục chứa các file CSV được tải lên. Server sẽ tự tạo nếu chưa tồn tại. |
| **Open Browser** | `--open-browser` | `false` | Tự động mở trình duyệt mặc định sau khi server khởi động thành công. |
| **Log Level** | `--log-level` | `info` | Mức độ log chi tiết của web server. |

### 3. Giới hạn hệ thống (System Limits)

Đây là các giới hạn cứng (hard-coded) được thiết lập để đảm bảo độ ổn định và tránh tràn bộ nhớ (OOM) cho Server khi phân tích dữ liệu lớn.

| Giới hạn | Giá trị | Mô tả |
| :--- | :--- | :--- |
| **Max Upload Size** | `200 MB` | Kích thước tối đa cho mỗi file upload qua giao diện Web. |
| **Max Loaded Files** | `20 Files` | Số lượng file tối đa được server load vào bộ nhớ RAM đồng thời để phân tích. |
| **Max Data Rows** | `5,000,000` | Số dòng dữ liệu tối đa cho phép trong một file CSV duy nhất. |
| **Max Process Size** | `200 MB` | Kích thước file vật lý tối đa mà server chấp nhận xử lý (kể cả file copy thủ công vào folder upload). |
| **File Format** | `.csv` | Chỉ chấp nhận định dạng CSV chuẩn do Client unostat sinh ra. |
| **Upload ID** | `UUID` | Tên file trên server được tự động thêm hậu tố UUID (Thay vì timestamp) để tránh xung đột tuyệt đối khi nhiều người upload cùng lúc. |

### Lưu ý quan trọng
*   Trên **Windows**, chỉ số `iowait` của CPU không khả dụng (luôn trả về -1) do hạn chế của hệ điều hành.
*   Tool cần quyền Admin/Root để đọc một số chỉ số phần cứng đặc biệt (ví dụ Disk I/O chi tiết trên một số distro Linux).

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).

&copy; 2026 UnoStat. Developed by Nguyen Thanh Phuong.
