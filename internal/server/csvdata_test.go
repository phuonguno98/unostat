/*
 * MIT License
 *
 * Copyright (c) 2026 Nguyen Thanh Phuong
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package server

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCSVDataService_LoadAndGet(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "unostat_csv_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	csvContent := `Timestamp,CPU,Memory
2023-10-26 10:00:00,10.0,50.0
2023-10-26 10:00:01,N/A,51.0
2023-10-26 10:00:02,12.0,52.0
2023-10-26 10:00:03,13.0,53.0
`
	filePath := filepath.Join(tempDir, "test.csv")
	if err := os.WriteFile(filePath, []byte(csvContent), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewCSVDataService(logger)

	// 1. Load File
	err = service.LoadFile("file1", "Test File", filePath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	// Verify metadata
	file, ok := service.GetFile("file1")
	if !ok {
		t.Fatal("GetFile() returned false")
	}
	if file.RowCount != 4 {
		t.Errorf("RowCount = %d, want 4", file.RowCount)
	}
	if len(file.Columns) != 3 {
		t.Errorf("Columns count = %d, want 3", len(file.Columns))
	}

	// 2. Get Data (Full Range)
	data, err := service.GetColumnData("file1", "CPU", nil, nil)
	if err != nil {
		t.Fatalf("GetColumnData(CPU) error = %v", err)
	}
	// Expect 3 points (row 2 is N/A for CPU)
	if len(data) != 3 {
		t.Errorf("DataPoints count = %d, want 3", len(data))
	}
	if data[0].Value != 10.0 {
		t.Errorf("Data[0] = %f, want 10.0", data[0].Value)
	}

	// 3. Get Data (Time Range)
	t1, err := time.Parse("2006-01-02 15:04:05", "2023-10-26 10:00:02")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := time.Parse("2006-01-02 15:04:05", "2023-10-26 10:00:03")
	if err != nil {
		t.Fatal(err)
	}

	// Range: Includes 10:00:02 and 10:00:03?
	// Binary search logic: Start >= t1, End > t2 (strictly >)
	// So to include t2, we need To be > t2. Wait, check logic:
	/*
		idx := sort.Search(..., func(i int) bool { return ts[i] > target })
		// if target=10:00:03, search finds index of first element > 10:00:03.
		// So it includes 10:00:03.
	*/

	dataRange, err := service.GetColumnData("file1", "Memory", &t1, &t2)
	if err != nil {
		t.Fatalf("GetColumnData(Memory, range) error = %v", err)
	}
	// Timestamps: 00, 01, 02, 03.
	// t1=02. StartIdx -> Finds 02.
	// t2=03. EndIdx -> Finds element > 03 (none, end of slice).
	// So range is [02, 03]. Count = 2.

	if len(dataRange) != 2 {
		t.Errorf("DataRange count = %d, want 2", len(dataRange))
	}
	if dataRange[0].Value != 52.0 {
		t.Errorf("DataRange[0] = %f, want 52.0", dataRange[0].Value)
	}

	// 4. Test NaN Handling (N/A)
	// Already implicitly tested in step 2 (CPU has N/A).

	// 5. Test Non-Existent Column
	_, err = service.GetColumnData("file1", "InvalidCol", nil, nil)
	if err == nil {
		t.Error("GetColumnData(InvalidCol) expected error")
	}

	// 6. Delete File
	err = service.DeleteFile("file1")
	if err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
	if _, ok := service.GetFile("file1"); ok {
		t.Error("File still exists after delete")
	}
}

func TestCSVDataService_InvalidFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_csv_invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewCSVDataService(logger)

	// Empty File
	emptyPath := filepath.Join(tempDir, "empty.csv")
	if err := os.WriteFile(emptyPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	err = service.LoadFile("empty", "Empty", emptyPath)
	if err == nil {
		t.Error("LoadFile(empty) expected error")
	}

	// No Data Rows
	headerOnlyPath := filepath.Join(tempDir, "header.csv")
	if err := os.WriteFile(headerOnlyPath, []byte("Time,Val"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = service.LoadFile("header", "Header", headerOnlyPath)
	if err == nil {
		t.Error("LoadFile(headerOnly) expected error")
	}
	if err != nil && !strings.Contains(err.Error(), "no valid data rows") {
		t.Errorf("LoadFile(headerOnly) wrong error: %v", err)
	}

	// Invalid Timestamp
	badTimePath := filepath.Join(tempDir, "badtime.csv")
	if err := os.WriteFile(badTimePath, []byte("Time,Val\nInvalidTime,10"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = service.LoadFile("badtime", "BadTime", badTimePath)
	// It skips rows with invalid timestamps logic, so loop finishes with rowCount 0 (effective data points).
	if err == nil {
		t.Error("LoadFile(badTime) expected error due to no valid rows")
	}
}
func TestParseTimestamp(t *testing.T) {
	ts := parseTimestamp("2023-10-26 12:00:00")
	if ts.IsZero() {
		t.Error("parseTimestamp valid failed")
	}

	tsInvalid := parseTimestamp("invalid")
	if !tsInvalid.IsZero() {
		t.Error("parseTimestamp invalid should return zero")
	}
}

func TestCSVDataService_LazyLoading(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_lazy_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	csvContent := "Timestamp,Val\n2023-01-01 00:00:00,10.0"
	filePath := filepath.Join(tempDir, "lazy.csv")
	if err := os.WriteFile(filePath, []byte(csvContent), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewCSVDataService(logger)

	// Register without loading
	service.RegisterFile("lazy1", "Lazy File", filePath)

	file, ok := service.GetFile("lazy1")
	if !ok {
		t.Fatal("File not registered")
	}
	if file.IsLoaded {
		t.Error("File should not be loaded yet")
	}

	// Load content
	err = service.LoadFileContent("lazy1")
	if err != nil {
		t.Fatalf("LoadFileContent failed: %v", err)
	}

	file, _ = service.GetFile("lazy1")
	if !file.IsLoaded {
		t.Error("File should be loaded now")
	}
	if file.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", file.RowCount)
	}
}

func TestCSVDataService_MaxFilesLimit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_limit_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewCSVDataService(logger)

	// Create a dummy CSV
	path := filepath.Join(tempDir, "dummy.csv")
	if err := os.WriteFile(path, []byte("Timestamp,Val\n2023-01-01 00:00:00,1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load MaxFiles (20)
	for i := 0; i < MaxFiles; i++ {
		id := fmt.Sprintf("file_%d", i)
		if err := service.LoadFile(id, id, path); err != nil {
			t.Fatalf("Failed to load file %d: %v", i, err)
		}
	}

	// Try to load one more
	err = service.LoadFile("overflow", "overflow", path)
	if err == nil {
		t.Error("Expected error when exceeding MaxFiles")
	} else if !strings.Contains(err.Error(), "maximum number") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestCSVDataService_Downsampling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_downsample_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create CSV with 2500 rows > 2000 max points
	var sb strings.Builder
	sb.WriteString("Timestamp,Val\n")

	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2500; i++ {
		tStr := baseTime.Add(time.Duration(i) * time.Second).Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("%s,%d.0\n", tStr, i))
	}

	path := filepath.Join(tempDir, "large.csv")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewCSVDataService(logger)

	if err := service.LoadFile("large", "Large", path); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Get data
	data, err := service.GetColumnData("large", "Val", nil, nil)
	if err != nil {
		t.Fatalf("GetColumnData failed: %v", err)
	}

	// Should be downsampled to 2000
	if len(data) != 2000 {
		t.Errorf("Downsampled count = %d, want 2000", len(data))
	}

	// Verify first and last value approximate range
	// Check first bucket (approx 0)
	if data[0].Value > 5 {
		t.Errorf("First value too high: %f", data[0].Value)
	}
}
