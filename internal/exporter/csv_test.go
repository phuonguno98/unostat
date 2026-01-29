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

package exporter

import (
	"context"
	"encoding/csv"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/phuonguno98/unostat/internal/config"
	"github.com/phuonguno98/unostat/pkg/metrics"
)

func TestCSVExporter_Export(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "unostat_exporter_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	outputPath := filepath.Join(tempDir, "export_test.csv")
	metricsChan := make(chan *metrics.Snapshot, 10)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		OutputPath:       outputPath,
		Timezone:         "UTC",
		FlushInterval:    100 * time.Millisecond,
		BufferSize:       10,
		SamplingInterval: 1 * time.Second,
	}

	exporter, err := NewCSVExporter(cfg, metricsChan, logger)
	if err != nil {
		t.Fatalf("NewCSVExporter() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)

	go func() {
		done <- exporter.Start(ctx)
	}()

	// Create a snapshot
	now := time.Date(2023, 10, 26, 12, 0, 0, 0, time.UTC)
	snapshot := &metrics.Snapshot{
		Timestamp: now,
		CPU:       45.5,
		CPUWait:   2.5,
		Memory:    60.0,
		Disks: map[string]metrics.DiskStats{
			"sda": {Utilization: 10.5, Await: 5.0, IOPS: 100.0},
		},
		Networks: map[string]metrics.NetStats{
			"eth0": {Bandwidth: 10_000_000}, // 10 Mbps
		},
	}

	metricsChan <- snapshot

	// Give it a moment to process
	time.Sleep(200 * time.Millisecond)

	// Close context
	cancel()
	err = <-done
	if err != nil {
		t.Errorf("Exporter finished with error: %v", err)
	}
	if err := exporter.Close(); err != nil {
		t.Errorf("Failed to close exporter: %v", err)
	}

	// Verify File Content
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Failed to close file: %v", err)
		}
	}()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records (Header + 1 Row), got %d", len(records))
	}

	// Check Header
	expectedHeader := []string{
		"Timestamp",
		"CPU Utilization (%)",
		"CPU IO Wait (%)",
		"Memory Utilization (%)",
		"Disk [sda] Utilization (%)",
		"Disk [sda] Average Wait (ms)",
		"Disk [sda] Throughput (IOPS)",
		"Network [eth0] Throughput (Mbps)",
	}

	header := records[0]
	if len(header) != len(expectedHeader) {
		t.Fatalf("Header length mismatch. Got %d, want %d", len(header), len(expectedHeader))
	}
	for i, h := range header {
		if h != expectedHeader[i] {
			t.Errorf("Header[%d] = %q, want %q", i, h, expectedHeader[i])
		}
	}

	// Check Data Row
	row := records[1]
	// Timestamp 2023-10-26 12:00:00, CPU 45.50, Wait 2.50, Mem 60.00, Disk 10.50, Wait 5.00, IOPS 100.00, Net 10.00
	expectedRow := []string{
		"2023-10-26 12:00:00",
		"45.50",
		"2.50",
		"60.00",
		"10.50",
		"5.00",
		"100.00",
		"10.00",
	}

	for i, v := range row {
		if v != expectedRow[i] {
			t.Errorf("Row[%d] = %q, want %q", i, v, expectedRow[i])
		}
	}
}

func TestCSVExporter_NA_Handling(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "unostat_exporter_na_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	outputPath := filepath.Join(tempDir, "export_na.csv")
	metricsChan := make(chan *metrics.Snapshot, 10)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		OutputPath:       outputPath,
		Timezone:         "UTC",
		FlushInterval:    100 * time.Millisecond,
		BufferSize:       10,
		SamplingInterval: 1 * time.Second,
	}

	exporter, err := NewCSVExporter(cfg, metricsChan, logger)
	if err != nil {
		t.Fatalf("NewCSVExporter() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)

	go func() {
		done <- exporter.Start(ctx)
	}()

	// Snapshot 1: Defines structure (sda)
	metricsChan <- &metrics.Snapshot{
		Timestamp: time.Now(),
		CPU:       10, CPUWait: 1, Memory: 10,
		Disks:    map[string]metrics.DiskStats{"sda": {}},
		Networks: map[string]metrics.NetStats{},
	}

	// Snapshot 2: Missing sda (should be N/A)
	metricsChan <- &metrics.Snapshot{
		Timestamp: time.Now(),
		CPU:       10, CPUWait: -1, // N/A CPUWait
		Memory:   10,
		Disks:    map[string]metrics.DiskStats{}, // No disks
		Networks: map[string]metrics.NetStats{},
	}

	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done
	if err := exporter.Close(); err != nil {
		t.Errorf("Failed to close exporter: %v", err)
	}

	// Verify
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Failed to close file: %v", err)
		}
	}()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 3 {
		t.Fatalf("Expected 3 records, got %d", len(records))
	}

	// Check 2nd row (Snapshot 2) for N/A
	row2 := records[2]
	// CPUWait column index is 2
	if row2[2] != naString {
		t.Errorf("Expected CPUWait to be N/A, got %q", row2[2])
	}
	// Disk columns should be at end.
	// Header: Time, CPU, Wait, Mem, DiskUtil, DiskWait
	// Indexes: 0, 1, 2, 3, 4, 5
	if len(row2) < 6 {
		t.Fatalf("Row 2 too short")
	}
	if row2[4] != naString || row2[5] != naString {
		t.Errorf("Expected Disk stats to be N/A, got %q, %q", row2[4], row2[5])
	}
}

func TestCSVExporter_FileRotation(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "unostat_rotation_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	outputPath := filepath.Join(tempDir, "rotation_test.csv")
	metricsChan := make(chan *metrics.Snapshot, 10)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		OutputPath:       outputPath,
		Timezone:         "UTC",
		FlushInterval:    100 * time.Millisecond,
		BufferSize:       10,
		SamplingInterval: 1 * time.Second,
	}

	exporter, err := NewCSVExporter(cfg, metricsChan, logger)
	if err != nil {
		t.Fatalf("NewCSVExporter() error = %v", err)
	}

	// Manually set size to trigger rotation
	exporter.currentSize = config.DefaultMaxOutputFileSize + 1

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)

	go func() {
		done <- exporter.Start(ctx)
	}()

	// Send snapshot to trigger rotation
	snapshot := &metrics.Snapshot{
		Timestamp: time.Now(),
		CPU:       50.0,
		CPUWait:   1.0,
		Memory:    70.0,
		Disks:     map[string]metrics.DiskStats{"sda": {Utilization: 20.0, Await: 10.0, IOPS: 150.0}},
		Networks:  map[string]metrics.NetStats{"eth0": {Bandwidth: 20_000_000}},
	}
	metricsChan <- snapshot

	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	if err := exporter.Close(); err != nil {
		t.Errorf("Failed to close exporter: %v", err)
	}

	// Verify rotated file exists
	rotatedPath := filepath.Join(tempDir, "rotation_test_1.csv")
	if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
		t.Errorf("Rotated file does not exist: %s", rotatedPath)
	}

	// Verify rotated file has header
	f, err := os.Open(rotatedPath)
	if err != nil {
		t.Fatalf("Failed to open rotated file: %v", err)
	}
	defer func() { _ = f.Close() }()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read rotated CSV: %v", err)
	}

	if len(records) < 1 {
		t.Fatal("Rotated file should have at least a header")
	}
}

func TestCSVExporter_FileRotation_NoOverwrite(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "unostat_rotation_overwrite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	outputPath := filepath.Join(tempDir, "overwrite_test.csv")

	// Create pre-existing rotated files
	existingFile1 := filepath.Join(tempDir, "overwrite_test_1.csv")
	if err := os.WriteFile(existingFile1, []byte("existing data 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	metricsChan := make(chan *metrics.Snapshot, 10)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		OutputPath:       outputPath,
		Timezone:         "UTC",
		FlushInterval:    100 * time.Millisecond,
		BufferSize:       10,
		SamplingInterval: 1 * time.Second,
	}

	exporter, err := NewCSVExporter(cfg, metricsChan, logger)
	if err != nil {
		t.Fatalf("NewCSVExporter() error = %v", err)
	}

	// Manually set size to trigger rotation
	exporter.currentSize = config.DefaultMaxOutputFileSize + 1

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)

	go func() {
		done <- exporter.Start(ctx)
	}()

	// Send snapshot to trigger rotation
	snapshot := &metrics.Snapshot{
		Timestamp: time.Now(),
		CPU:       50.0,
		CPUWait:   1.0,
		Memory:    70.0,
		Disks:     map[string]metrics.DiskStats{},
		Networks:  map[string]metrics.NetStats{},
	}
	metricsChan <- snapshot

	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	if err := exporter.Close(); err != nil {
		t.Errorf("Failed to close exporter: %v", err)
	}

	// Verify original file still has old content
	oldContent, err := os.ReadFile(existingFile1)
	if err != nil {
		t.Fatal(err)
	}
	if string(oldContent) != "existing data 1" {
		t.Error("Original file was overwritten")
	}

	// Verify new file was created with index 2
	newFile := filepath.Join(tempDir, "overwrite_test_2.csv")
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Errorf("New rotated file with index 2 should exist: %s", newFile)
	}
}

func TestCSVExporter_InvalidTimezone(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_tz_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	outputPath := filepath.Join(tempDir, "test.csv")
	metricsChan := make(chan *metrics.Snapshot, 10)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		OutputPath:       outputPath,
		Timezone:         "Invalid/Timezone",
		FlushInterval:    100 * time.Millisecond,
		BufferSize:       10,
		SamplingInterval: 1 * time.Second,
	}

	_, err = NewCSVExporter(cfg, metricsChan, logger)
	if err == nil {
		t.Error("Expected error for invalid timezone, got nil")
	}
}
