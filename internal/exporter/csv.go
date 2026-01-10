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
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/phuonguno98/unostat/internal/config"
	"github.com/phuonguno98/unostat/pkg/metrics"
)

// CSVExporter exports metrics to a CSV file with buffering.
type CSVExporter struct {
	config        *config.Config
	file          *os.File
	csvWriter     *csv.Writer
	bufWriter     *bufio.Writer
	metricsChan   <-chan *metrics.Snapshot
	flushTicker   *time.Ticker
	recordCount   int
	logger        *slog.Logger
	headerWritten bool
	deviceOrder   []string       // Track order of devices for consistent columns
	ifaceOrder    []string       // Track order of interfaces for consistent columns
	location      *time.Location // Timezone location for timestamps
	currentSize   int64          // Current file size in bytes
	basePath      string         // Base output path
	fileIndex     int            // Index for file rotation
}

// NewCSVExporter creates a new CSV exporter instance.
func NewCSVExporter(cfg *config.Config, metricsChan <-chan *metrics.Snapshot, logger *slog.Logger) (*CSVExporter, error) {
	// Parse timezone
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		// Fallback to Local if invalid, but log warning? Or return error?
		// User asked for "default Local", so if "Local" is passed, it works.
		// If an invalid string is passed, LoadLocation returns error.
		return nil, fmt.Errorf("invalid timezone '%s': %w", cfg.Timezone, err)
	}

	// Open output file
	file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %w", err)
	}

	// Create buffered writer
	bufWriter := bufio.NewWriterSize(file, 8192) // 8KB buffer

	// Create CSV writer on top of buffered writer
	csvWriter := csv.NewWriter(bufWriter)

	// Get initial file size (if appending)
	stat, err := file.Stat()
	if err != nil {
		file.Close() // Close if stat fails
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	exporter := &CSVExporter{
		config:      cfg,
		file:        file,
		csvWriter:   csvWriter,
		bufWriter:   bufWriter,
		metricsChan: metricsChan,
		logger:      logger,
		location:    loc,
		currentSize: stat.Size(),
		basePath:    cfg.OutputPath,
		fileIndex:   0,
	}

	return exporter, nil
}

// Start begins listening to the metrics channel and writing to CSV.
func (e *CSVExporter) Start(ctx context.Context) error {
	e.logger.Info("Starting CSV exporter", "output", e.config.OutputPath, "timezone", e.config.Timezone)

	// Start flush ticker
	e.flushTicker = time.NewTicker(e.config.FlushInterval)
	defer e.flushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("CSV exporter stopping...")
			return e.flush()

		case snapshot, ok := <-e.metricsChan:
			if !ok {
				// Channel closed, flush and exit
				e.logger.Info("Metrics channel closed, flushing remaining data...")
				return e.flush()
			}

			if err := e.writeSnapshot(snapshot); err != nil {
				e.logger.Error("Failed to write snapshot", "error", err)
			}

			e.recordCount++

			// Flush if buffer size reached
			if e.recordCount >= e.config.BufferSize {
				if err := e.flush(); err != nil {
					e.logger.Error("Failed to flush", "error", err)
				}
				e.recordCount = 0
			}

		case <-e.flushTicker.C:
			// Time-based flush
			if e.recordCount > 0 {
				if err := e.flush(); err != nil {
					e.logger.Error("Failed to flush", "error", err)
				}
				e.recordCount = 0
			}
		}
	}
}

// writeSnapshot writes a single snapshot to the CSV file.
func (e *CSVExporter) writeSnapshot(snapshot *metrics.Snapshot) error {
	// Write header if this is the first record
	if !e.headerWritten {
		if err := e.writeHeader(snapshot); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
		e.headerWritten = true
	}

	// Build row
	row := e.buildRow(snapshot)

	// Check for rotation if file size exceeds limit
	// We check *before* writing to avoid going too much over the limit
	// Note: We only rotate if we have written at least something to avoid empty file loops if logic is flawed,
	// checking currentSize > limit is enough.
	if e.currentSize >= config.DefaultMaxOutputFileSize {
		if err := e.rotateFile(snapshot); err != nil {
			e.logger.Error("Failed to rotate file", "error", err)
			// Continue writing to old file if rotation fails?
			// Ideally we should stop or retry, but logging error and continuing is safer than crashing.
		}
	}

	// Write row
	rowBytes := 0
	for _, cell := range row {
		rowBytes += len(cell) + 1 // +1 for comma
	}
	rowBytes += 1 // +1 for newline approximation

	if err := e.csvWriter.Write(row); err != nil {
		return fmt.Errorf("failed to write row: %w", err)
	}

	e.currentSize += int64(rowBytes) // Approximate size tracking
	return nil
}

// writeHeader writes the CSV header row.
func (e *CSVExporter) writeHeader(snapshot *metrics.Snapshot) error {
	header := []string{"Timestamp", "CPU Utilization (%)", "CPU IO Wait (%)"}
	header = append(header, "Memory Utilization (%)")

	// Extract and sort device names for consistent ordering
	e.deviceOrder = make([]string, 0, len(snapshot.Disks))
	for device := range snapshot.Disks {
		e.deviceOrder = append(e.deviceOrder, device)
	}
	sort.Strings(e.deviceOrder)

	// Add disk columns
	for _, device := range e.deviceOrder {
		header = append(header,
			fmt.Sprintf("Disk [%s] Utilization (%%)", device),
			fmt.Sprintf("Disk [%s] Average Wait (ms)", device),
			fmt.Sprintf("Disk [%s] Throughput (IOPS)", device))
	}

	// Extract and sort interface names for consistent ordering
	e.ifaceOrder = make([]string, 0, len(snapshot.Networks))
	for iface := range snapshot.Networks {
		e.ifaceOrder = append(e.ifaceOrder, iface)
	}
	sort.Strings(e.ifaceOrder)

	// Add network columns
	for _, iface := range e.ifaceOrder {
		header = append(header, fmt.Sprintf("Network [%s] Throughput (Mbps)", iface))
	}

	return e.csvWriter.Write(header)
}

// buildRow builds a CSV row from a snapshot.
func (e *CSVExporter) buildRow(snapshot *metrics.Snapshot) []string {
	// Convert timestamp to configured timezone
	ts := snapshot.Timestamp.In(e.location)

	row := []string{
		ts.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("%.2f", snapshot.CPU),
		e.formatCPUWait(snapshot.CPUWait),
		fmt.Sprintf("%.2f", snapshot.Memory),
	}

	// Add disk metrics in consistent order
	for _, device := range e.deviceOrder {
		if stats, ok := snapshot.Disks[device]; ok {
			row = append(row,
				fmt.Sprintf("%.2f", stats.Utilization),
				fmt.Sprintf("%.2f", stats.Await),
				fmt.Sprintf("%.2f", stats.IOPS))
		} else {
			row = append(row, naString, naString, naString)
		}
	}

	// Add network metrics in consistent order
	for _, iface := range e.ifaceOrder {
		if stats, ok := snapshot.Networks[iface]; ok {
			// Convert bits per second to Mbps
			mbps := stats.Bandwidth / 1_000_000
			row = append(row, fmt.Sprintf("%.2f", mbps))
		} else {
			row = append(row, naString)
		}
	}

	return row
}

const naString = "N/A"

// formatCPUWait formats CPU wait value, handling N/A case.
func (e *CSVExporter) formatCPUWait(cpuWait float64) string {
	if cpuWait < 0 {
		return naString
	}
	return fmt.Sprintf("%.2f", cpuWait)
}

// flush flushes the buffered data to disk.
func (e *CSVExporter) flush() error {
	e.csvWriter.Flush()
	if err := e.csvWriter.Error(); err != nil {
		return fmt.Errorf("CSV writer error: %w", err)
	}

	if err := e.bufWriter.Flush(); err != nil {
		return fmt.Errorf("buffer writer error: %w", err)
	}

	e.logger.Debug("Flushed to disk", "records", e.recordCount)
	return nil
}

// Close closes the CSV exporter and flushes remaining data.
func (e *CSVExporter) Close() error {
	e.logger.Info("Closing CSV exporter")

	if e.flushTicker != nil {
		e.flushTicker.Stop()
	}

	// Final flush
	if err := e.flush(); err != nil {
		e.logger.Error("Final flush failed", "error", err)
	}

	// Close file
	if err := e.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	e.logger.Info("CSV exporter closed")
	return nil
}

// rotateFile rotates the output file.
// rotateFile rotates the output file.
func (e *CSVExporter) rotateFile(snapshot *metrics.Snapshot) error {
	e.logger.Info("Rotating output file", "current_size", e.currentSize)

	// Flush and close current file
	if err := e.flush(); err != nil {
		return fmt.Errorf("flush before rotate failed: %w", err)
	}
	if err := e.file.Close(); err != nil {
		return fmt.Errorf("close before rotate failed: %w", err)
	}

	// Generate new filename with collision existence check
	ext := filepath.Ext(e.basePath)
	base := strings.TrimSuffix(e.basePath, ext)
	var newPath string

	for {
		e.fileIndex++
		newPath = fmt.Sprintf("%s_%d%s", base, e.fileIndex, ext)
		// Check if file exists to avoid overwriting previous run data or manual files
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			break
		}
		// If exists, loop will continue and increment index
	}

	// Open new file
	file, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open new rotated file: %w", err)
	}

	// Update exporter state
	e.file = file
	e.bufWriter = bufio.NewWriterSize(file, 8192)
	e.csvWriter = csv.NewWriter(e.bufWriter)
	e.currentSize = 0
	e.headerWritten = false

	// Write header to new file immediately
	if err := e.writeHeader(snapshot); err != nil {
		return fmt.Errorf("failed to write header to rotated file: %w", err)
	}
	e.headerWritten = true

	e.logger.Info("File rotated successfully", "new_path", newPath)
	return nil
}
