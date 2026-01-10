# Tài Liệu Thiết Kế - UnoStat

---

## 1. Tổng Quan Hệ Thống

### 1.1. Mục Đích

**UnoStat** là công cụ giám sát hiệu năng hệ thống gọn nhẹ, đa nền tảng được viết bằng Go. Chuyên biệt cho **Kiểm thử hiệu năng (Performance Testing)** để theo dõi thời gian thực CPU, RAM, Disk Utilization (busy time), Await và Băng thông mạng (Network Bandwidth) với khả năng xuất dữ liệu CSV.

### 1.2. Nguyên Lý Cốt Lõi

Hệ thống hoạt động dựa trên nguyên lý **Delta Sampling** (Lấy mẫu chênh lệch). Do các bộ đếm (counters) của hệ điều hành thường là giá trị tích lũy (monotonic increasing), UnoStat tính toán hiệu năng trong một khoảng thời gian $\Delta t$ bằng cách so sánh giá trị hiện tại với giá trị của lần lấy mẫu trước đó.

---

## 2. Kiến Trúc Phần Mềm

### 2.1. Mô Hình Tổng Quát

UnoStat sử dụng mô hình **Producer-Consumer** bất đồng bộ thông qua Go Channels để tách biệt quá trình thu thập (Collection) và quá trình ghi đĩa (Exporting). Điều này giúp các thread thu thập dữ liệu không bao giờ bị chặn (block) bởi tốc độ ghi đĩa, đảm bảo metrics luôn được lấy đúng thời điểm.

```
┌─────────────────────────────────────────────────────────────┐
│                       UnoStat Process                       │
│                                                             │
│  ┌──────────────┐      ┌─────────────┐      ┌────────────┐  │
│  │  Collector   │      │             │      │    CSV     │  │
│  │   Manager    │─────▶│   Metrics   │─────▶│  Exporter │  │
│  │  (Producer)  │      │   Channel   │      │ (Consumer) │  │
│  └──────────────┘      └─────────────┘      └────────────┘  │
│         │                                          │        │
│    (Ticker Loop)                              (Buffer I/O)  │
│         ▼                                          ▼        │
│   ┌───────────┐                              ┌───────────┐  │
│   │ OS Kernel │                              │ CSV File  │  │
│   └───────────┘                              └───────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2. Các Thành Phần Chính

#### A. Entry Point (`cmd/unostat`)
*   **Initialization:** Parse CLI flags, khởi tạo Logger (slog) và Config.
*   **Orchestration:** Kích hoạt Collector Manager và CSV Exporter.
*   **Signal Handling:** Lắng nghe tín hiệu hệ thống (`SIGINT`, `SIGTERM`) để shutdown an toàn, đảm bảo dữ liệu trong buffer được ghi hết xuống đĩa trước khi thoát.

#### B. Collector Manager (`internal/collector`)
*   **Vai trò:** Producer (Người sản xuất dữ liệu).
*   **Quy trình hoạt động:**
    1.  **Baseline Collection ($T_0$):** Chạy một lần ngay khi khởi động để lưu trữ trạng thái ban đầu của hệ thống. Dữ liệu này *không* được xuất ra file.
    2.  **Startup Delay:** Tạm dừng (1 giây) để ổn định trước khi bắt đầu chu kỳ lặp.
    3.  **Collection Loop:** Sử dụng `time.Ticker` (mặc định 30s) để kích hoạt thu thập.
    4.  **Delta Calculation:** Tại mỗi nhịp (`Tick`), gọi các sub-collectors (CPU, Disk, Net) để lấy dữ liệu mới và tính Delta với dữ liệu cũ.
    5.  **Non-blocking Send:** Gửi kết quả (Snapshot) vào Channel. Nếu Channel đầy (do Consumer xử lý chậm), Snapshot sẽ bị **hủy bỏ (Drop)** và ghi log cảnh báo, thay vì làm treo luồng thu thập.

#### C. Metrics Channel
*   **Loại:** Buffered Channel (`chan *metrics.Snapshot`).
*   **Dung lượng:** 10 slots.
*   **Mục đích:** Là bộ đệm trung gian, giúp Collector và Exporter hoạt động độc lập về tốc độ.

#### D. CSV Exporter (`internal/exporter`)
*   **Vai trò:** Consumer (Người tiêu thụ dữ liệu).
*   **Quy trình hoạt động:**
    1.  Tạo file CSV tại đường dẫn chỉ định.
    2.  Lắng nghe liên tục từ Metrics Channel.
    3.  Format dữ liệu và ghi vào **Memory Buffer** (sử dụng `bufio.Writer` với kích thước buffer cấu hình được).
    4.  **Flush Strategy:** Dữ liệu chỉ được ghi vật lý xuống đĩa khi:
        *   Buffer đầy.
        *   Đến chu kỳ Flush định kỳ (defaults to 5s).
        *   Nhận tín hiệu dừng ứng dụng.

---

## 3. Thiết Kế Chi Tiết & Quyết Định Kỹ Thuật

### 3.1. Daemon Mode (Chế Độ Chạy Ngầm)
Hiện tại (phiên bản 1.0.0), UnoStat hoạt động như một **Foreground Process**.
*   Mặc dù các cờ `--daemon` và `--pid-file` đã được định nghĩa trong cấu hình, nhưng tính năng tự tách process (forking) chưa được tích hợp trực tiếp vào binary lõi.
*   **Khuyên dùng:** Sử dụng các trình quản lý service của hệ điều hành để chạy UnoStat như một background service tin cậy:
    *   **systemd** (Linux)
    *   **Windows Service** (sc.exe / nssm)
    *   **launchd** (macOS)

### 3.2. Xử Lý Lỗi (Fault Tolerance)
*   **Partial Metrics:** Hệ thống được thiết kế để "chịu lỗi một phần". Ví dụ: nếu không lấy được thông tin Disk, metrics về CPU và RAM vẫn được thu thập và ghi lại bình thường.
*   **Channel Overflow Protection:** Ưu tiên tính "Real-time" hơn tính "Toàn vẹn tuyệt đối". Khi hệ thống quá tải (đĩa quá chậm), thà mất một vài điểm dữ liệu (drop snapshot) còn hơn làm treo tiến trình giám sát chính.

---

## 4. Cấu Trúc Dữ Liệu (Domain Models)

Dữ liệu được chuyển giữa các thành phần thông qua struct `Snapshot` đã qua xử lý (không phải raw counters).

```go
// Snapshot đại diện cho trạng thái hệ thống tại một thời điểm
type Snapshot struct {
    Timestamp time.Time
    CPU       float64             // % Utilization
    CPUWait   float64             // % IO Wait (-1 nếu không khả dụng)
    Memory    float64             // % RAM Used
    Disks     map[string]DiskStats // Chi tiết từng ổ đĩa
    Networks  map[string]NetStats  // Chi tiết từng card mạng
}
```

---

## 5. Hướng Phát Triển Tương Lai (Roadmap)
*   **Native Daemon:** Tích hợp thư viện `go-daemon` để hỗ trợ cờ `--daemon` trực tiếp mà không cần tool ngoài.
*   **REST API:** Thêm HTTP Server để expose metrics dạng JSON hoặc Prometheus format (`/metrics`).
*   **Container Awareness:** Tự động phát hiện môi trường Docker/K8s để chuyển sang đọc metrics từ Cgroups (hiện tại đang đọc metrics của Host Kernel thông qua `/proc`).
