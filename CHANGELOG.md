# Changelog

All notable changes to this project will be documented in this file.

## [v1.0.1] - 2026-01-29

### 🐛 Bug Fixes
*   **Collector Logic:** Fixed an issue where metrics were not being recorded when filters were applied due to incorrect baseline collection logic.
*   **Device Filtering:** Resolved inconsistency between `list-devices` output (`/dev/sdd`) and internal collector format (`sdd`). Both formats are now supported in `--include-disks` and `--exclude-disks`.
*   **Code Quality:** Fixed various linter issues (errcheck, revive) and improved test stability.

### 📦 Improvements
*   **Packaging:** Updated `Makefile` and CI/CD pipeline to include documentation, license, and changelog in release artifacts.
*   **CI/CD:** Added manual workflow triggers, explicit zip installation, and verbose logging for better build reliability.
*   **CLI:** Updated `list-devices` command to show correct usage examples with the `collect` subcommand.


## [v1.0.0] - 2026-01-10

**UnoStat** - The Lightweight System Performance Monitoring Tool is officially released!

### 🚀 Key Features

*   **Cross-Platform Monitoring:** Real-time tracking of system resources on Windows, Linux, and macOS.
*   **Performance Metrics Collection:**
    *   **CPU:** User, System, Idle, IOWait.
    *   **Memory:** Usage percentage.
    *   **Disk:** Utilization (Busy Time), Await latency, IOPS.
    *   **Network:** Total Bandwidth (In/Out).
*   **Robust CSV Export:**
    *   Automatic file rotation (splits files >150MB).
    *   Safe writing mechanism (flushes to disk periodically).
    *   No-overwrite protection for rotated files.
*   **Interactive Web Dashboard:**
    *   Visualize metrics with smooth, high-performance line charts.
    *   **Lazy Loading:** Optimizes startup time by loading CSV data only when requested.
    *   **Analysis Tools:** Zoom, Pan, Time Range Filter.
    *   **Statistics:** Real-time Min, Max, Average display for all metrics.
    *   **Export:** Download charts as high-quality PNGs or export all data as a TAR archive.
*   **CLI Tooling:**
    *   `collect`: Start metric collection agent.
    *   `visualize`: Launch the local web server.
    *   `list-devices`: Helper to identify disk and network interfaces.

### 🛠 Technical Improvements

*   **High Test Coverage:** >80% coverage for core modules (Metrics, Server, Exporter).
*   **CI/CD Pipeline:** Automated testing and release builds via GitHub Actions.
*   **Windows Integration:** Custom executable icon and resource embedding.
*   **Security:** Path traversal protection and filename sanitization for uploads.
