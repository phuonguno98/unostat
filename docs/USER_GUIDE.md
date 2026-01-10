# Hướng Dẫn Sử Dụng UnoStat CLI

**UnoStat** là công cụ giám sát hiệu năng hệ thống gọn nhẹ, đa nền tảng được viết bằng Go. Chuyên biệt cho **Kiểm thử hiệu năng (Performance Testing)** để theo dõi thời gian thực CPU, RAM, Disk Utilization (busy time), Await và Băng thông mạng (Network Bandwidth) với khả năng xuất dữ liệu CSV.

Tài liệu này cung cấp hướng dẫn chi tiết về cách cài đặt, các câu lệnh, tùy chọn cấu hình và các kịch bản sử dụng phổ biến.

---

## 1. Cài Đặt & Build

Trước khi sử dụng, bạn cần có file thực thi `unostat`. Bạn có thể biên dịch từ mã nguồn:

```bash
# Build phiên bản CLI
go build -o bin/unostat.exe ./cmd/unostat
```

File thực thi sẽ nằm tại `bin/unostat.exe` (Windows) hoặc `bin/unostat` (Linux/macOS).

## 2. Các Lệnh Cơ Bản

Cấu trúc lệnh chung:
```bash
unostat [command] [flags]
```

### 2.1. Bắt Đầu Giám Sát (`collect`)

Sử dụng lệnh `collect` để bắt đầu quá trình thu thập métrics.

```bash
./bin/unostat collect [flags]
```

Mặc định:
- Interval: 30 giây.
- Output: File CSV tự động đặt tên theo format `<hostname>_<timestamp>.csv`.
- Ghi log ra màn hình console (stdout).


### 2.3. Liệt Kê Thiết Bị (`list-devices`)

Trước khi cấu hình giám sát, bạn nên xem danh sách các thiết bị ổ đĩa và card mạng mà UnoStat nhận diện được. Điều này giúp bạn lấy tên chính xác để cấu hình bộ lọc (include/exclude).

```bash
./bin/unostat list-devices
```

### 2.4. Kiểm Tra Phiên Bản (`version`)

Xem thông tin phiên bản, commit hash và ngày build của ứng dụng.

```bash
./bin/unostat version
```

### 2.5. Phân Tích Dữ Liệu (`visualize`)

Khởi chạy Dashboard web để xem biểu đồ phân tích dữ liệu từ file CSV.

```bash
./bin/unostat visualize [flags]
```

Ví dụ:
```bash
./bin/unostat visualize --port 8080 --open-browser
```

---

## 3. Tùy Chọn Cấu Hình (Flags)

Bạn có thể thay đổi hành vi của UnoStat thông qua các cờ (flags) khi chạy lệnh giám sát.

### Cấu Hình Chung

| Flag | Kiểu | Mặc định | Mô tả |
|------|------|----------|-------|
| `--interval` | Duration | `30s` | Khoảng thời gian lấy mẫu. Vd: `1s`, `500ms`, `1m`. |
| `--output` | String | `auto` | Đường dẫn file CSV đầu ra. Mặc định là `<hostname>_<timestamp>.csv`. |
| `--timezone` | String | `Local` | Múi giờ sử dụng cho timestamp trong file CSV. Vd: `Asia/Ho_Chi_Minh`. |


> **Lưu ý:** Flag `--timezone`, `--log-level`, `--log-file` là **Global Flags** (có thể dùng cho mọi lệnh), nhưng chủ yếu tác dụng với `collect`.

### Cấu Hình Logging & Buffer

| Flag | Kiểu | Mặc định | Mô tả |
|------|------|----------|-------|
| `--log-level` | String | `info` | Mức độ chi tiết log: `debug`, `info`, `warn`, `error`. |
| `--log-file` | String | `stdout` | Đường dẫn file log. Nếu để trống sẽ ghi ra màn hình. |
| `--buffer-size`| Int | `100` | Số lượng bản ghi giữ trong bộ nhớ đệm trước khi ghi xuống đĩa. |
| `--flush-interval`| Duration| `5s` | Khoảng thời gian định kỳ ghi dữ liệu từ bộ nhớ đệm xuống đĩa. |

### Tùy Chọn Visualizer (Lệnh `visualize`)

| Flag | Kiểu | Mặc định | Mô tả |
|------|------|----------|-------|
| `--port` | Int | `8080` | Port lắng nghe của Web Server. |
| `--host` | String | `0.0.0.0` | Địa chỉ IP lắng nghe (0.0.0.0 = public). |
| `--upload-dir` | String | `(exe_dir)/uploads` | Thư mục lưu file CSV upload lên. |
| `--open-browser` | Bool | `false` | Tự động mở trình duyệt mặc định. |


### Bộ Lọc Thiết Bị (Filtering)

UnoStat cho phép chọn lọc cụ thể các thiết bị cần giám sát để giảm nhiễu dữ liệu.

| Flag | Mô tả | Ví dụ |
|------|-------|-------|
| `--include-disks` | Danh sách tên ổ đĩa cần giám sát (ngăn cách bởi dấu phẩy). | `"C:,D:"` hoặc `"/dev/sda"` |
| `--exclude-disks` | Danh sách tên ổ đĩa cần loại bỏ. | `"E:"` hoặc `"/dev/loop0"` |
| `--include-networks`| Danh sách card mạng cần giám sát. | `"Ethernet,Wi-Fi"` |
| `--exclude-networks`| Danh sách card mạng cần loại bỏ. | `"Loopback,vEthernet"` |

> **Lưu ý:**
> - Nếu không chỉ định `include`, mặc định sẽ giám sát TẤT CẢ thiết bị (trừ những cái bị `exclude`).
> - `exclude` có độ ưu tiên cao hơn `include`.

---

## 4. Các Ví Dụ Sử Dụng (Use Cases)

### Kịch bản 1: Giám sát nhanh với tần suất cao (High frequency)
Chạy giám sát mỗi 1 giây, ghi kết quả ra file cụ thể.

```bash
./bin/unostat collect --interval 1s --output ./results/test_run_01.csv
```

### Kịch bản 2: Load Testing - Chỉ giám sát Disk và Network quan trọng
Loại bỏ các ổ đĩa ảo hoặc card mạng không cần thiết để tập trung vào tài nguyên thực tế.

```bash
./bin/unostat collect --interval 5s \
  --include-disks "C:" \
  --include-networks "Ethernet" \
  --log-level error
```



### Kịch bản 4: Debug và kiểm tra
Bật log mức `debug` để xem chi tiết quá trình thu thập dữ liệu.

```bash
./bin/unostat collect --log-level debug
```

---

## 5. Định Dạng Dữ Liệu Đầu Ra (CSV)

File CSV được tạo ra sẽ có các cột dữ liệu theo thời gian thực:
- **Timestamp**: Thời gian thu thập (theo Timezone cấu hình).
- **CPU**: Tổng % sử dụng (Usage), User%, System%, Idle%...
- **Memory**: Total, Used, Free, UsedPercent.
- **Disk**: Với mỗi Disk được giám sát sẽ có các cột: `Utilization` (Busy Time %), `Average Wait` (ms), `Throughput` (IOPS).
- **Network**: Với mỗi Interface được giám sát sẽ có cột: `Throughput` (Mbps).

Dữ liệu này có thể được import trực tiếp vào Excel, Google Sheets, hoặc các công cụ vẽ biểu đồ (Pandas/Matplotlib) để phân tích bottleneck hệ thống.
