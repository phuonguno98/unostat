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
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// MaxFileSize limits CSV file size to prevent memory issues (200MB)
	MaxFileSize = 200 * 1024 * 1024
	// MaxFiles limits number of loaded files
	MaxFiles = 20
	// MaxRowsPerFile limits rows per file to prevent memory issues
	MaxRowsPerFile = 5000000
)

// CSVFile represents a parsed CSV file with metadata.
type CSVFile struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Columns  []string  `json:"columns"`
	RowCount int       `json:"rowCount"`
	MinTime  time.Time `json:"minTime"`
	MaxTime  time.Time `json:"maxTime"`
	IsLoaded bool      `json:"isLoaded"`
}

// DataPoint represents a single data point in a time series.
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// ColumnData holds parsed data in columnar format for efficient storage and access.
type ColumnData struct {
	Timestamps []int64              // Unix timestamps for fast searching/filtering
	Values     map[string][]float64 // Map column name to slice of values (aligned with Timestamps)
}

// CSVDataService manages CSV files and provides data access.
type CSVDataService struct {
	files      map[string]*CSVFile
	columnData map[string]*ColumnData
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewCSVDataService creates a new CSV data service.
func NewCSVDataService(logger *slog.Logger) *CSVDataService {
	return &CSVDataService{
		files:      make(map[string]*CSVFile),
		columnData: make(map[string]*ColumnData),
		logger:     logger,
	}
}

// LoadFile loads a CSV file into the service.
func (s *CSVDataService) LoadFile(id, name, path string) error {
	// 1. Initial check (Read Lock)
	s.mu.RLock()
	if len(s.files) >= MaxFiles {
		// Only check if we are adding a new file that will be LOADED.
		// If we are fully loading, we consume memory.
		s.mu.RUnlock()
		return fmt.Errorf("maximum number of files reached (%d), please delete some files first", MaxFiles)
	}
	s.mu.RUnlock()

	// Forward to internal processing
	parsedCols, fileMeta, err := s.processCSVFile(id, name, path)
	if err != nil {
		return err
	}
	fileMeta.IsLoaded = true

	// 3. Update state (Write Lock)
	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check limit in case race condition added files while we were processing
	// NOTE: This check is a bit simplistic if we mix loaded and unloaded files in the same map.
	// Ideally we count loaded files only.
	loadedCount := 0
	for _, f := range s.files {
		if f.IsLoaded {
			loadedCount++
		}
	}
	if loadedCount >= MaxFiles {
		return fmt.Errorf("maximum number of loaded files reached (%d) during processing", MaxFiles)
	}

	s.files[id] = fileMeta
	s.columnData[id] = parsedCols

	return nil
}

// RegisterFile adds a file to the registry without loading its content.
// This supports lazy loading scenarios.
func (s *CSVDataService) RegisterFile(id, name, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.files[id]; !exists {
		s.files[id] = &CSVFile{
			ID:       id,
			Name:     name,
			Path:     path,
			IsLoaded: false,
		}
	}
}

// LoadFileContent loads the actual CSV data for a registered file into memory.
// It respects the MaxFiles limit for loaded files.
func (s *CSVDataService) LoadFileContent(id string) error {
	s.mu.RLock()
	fileMeta, exists := s.files[id]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("file not found: %s", id)
	}

	if fileMeta.IsLoaded {
		return nil // Already loaded
	}

	// Check limits
	s.mu.RLock()
	// count loaded files
	loadedCount := 0
	for _, f := range s.files {
		if f.IsLoaded {
			loadedCount++
		}
	}
	s.mu.RUnlock()

	if loadedCount >= MaxFiles {
		return fmt.Errorf("maximum number of loaded files reached (%d)", MaxFiles)
	}

	parsedCols, newMeta, err := s.processCSVFile(id, fileMeta.Name, fileMeta.Path)
	if err != nil {
		return err
	}
	newMeta.IsLoaded = true

	s.mu.Lock()
	defer s.mu.Unlock()

	s.files[id] = newMeta
	s.columnData[id] = parsedCols

	return nil
}

// processCSVFile reads and parses the CSV file into columnar format.
func (s *CSVDataService) processCSVFile(id, name, path string) (*ColumnData, *CSVFile, error) {
	// Check file size
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if fileInfo.Size() > MaxFileSize {
		return nil, nil, fmt.Errorf("file too large (max %d MB)", MaxFileSize/(1024*1024))
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.Error("failed to close file", "path", path, "error", err)
		}
	}()

	reader := csv.NewReader(file)
	reader.ReuseRecord = true

	// Read header
	headerRow, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	// Make a copy of the header because ReuseRecord is enabled
	header := make([]string, len(headerRow))
	copy(header, headerRow)

	colCount := len(header)
	if colCount < 2 {
		return nil, nil, fmt.Errorf("CSV must have at least timestamp and one data column")
	}

	// Initialize columnar storage
	timestamps := make([]int64, 0, 1000) // Pre-allocate with a guess
	valueCols := make(map[string][]float64)
	for i := 1; i < colCount; i++ {
		valueCols[header[i]] = make([]float64, 0, 1000)
	}

	rowCount := 0
	var minTime, maxTime time.Time

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error reading CSV line %d: %w", rowCount+2, err)
		}

		if rowCount >= MaxRowsPerFile {
			return nil, nil, fmt.Errorf("file has too many rows (max %d)", MaxRowsPerFile)
		}

		// Parse Timestamp (Column 0)
		t := parseTimestamp(record[0])
		if t.IsZero() {
			continue
		}

		timestamps = append(timestamps, t.Unix())

		// Track min/max time
		if rowCount == 0 {
			minTime = t
			maxTime = t
		} else {
			if t.Before(minTime) {
				minTime = t
			}
			if t.After(maxTime) {
				maxTime = t
			}
		}

		// Parse Values (Columns 1..N)
		for i := 1; i < colCount; i++ {
			colName := header[i]
			valStr := strings.TrimSpace(record[i])
			var val float64
			if valStr == "" || valStr == "N/A" {
				val = math.NaN()
			} else {
				v, err := strconv.ParseFloat(valStr, 64)
				if err != nil {
					val = math.NaN()
				} else {
					val = v
				}
			}
			valueCols[colName] = append(valueCols[colName], val)
		}

		rowCount++
	}

	if rowCount == 0 {
		return nil, nil, fmt.Errorf("CSV file contains no valid data rows")
	}

	parsedCols := &ColumnData{
		Timestamps: timestamps,
		Values:     valueCols,
	}

	fileMeta := &CSVFile{
		ID:       id,
		Name:     name,
		Path:     path,
		Columns:  header,
		RowCount: rowCount,
		MinTime:  minTime,
		MaxTime:  maxTime,
	}

	return parsedCols, fileMeta, nil
}

// GetFiles returns all loaded CSV files.
func (s *CSVDataService) GetFiles() []*CSVFile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]*CSVFile, 0, len(s.files))
	for _, f := range s.files {
		files = append(files, f)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	return files
}

// GetFile returns a specific CSV file by ID.
func (s *CSVDataService) GetFile(id string) (*CSVFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.files[id]
	return file, ok
}

// GetColumnData returns time series data for a specific column with optional time filtering.
// It automatically downsamples data if the number of points exceeds maxPoints (default 2000).
func (s *CSVDataService) GetColumnData(fileID, columnName string, timeFrom, timeTo *time.Time) ([]DataPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Check if file/data exists
	colsData, ok := s.columnData[fileID]
	if !ok {
		return nil, fmt.Errorf("data not found for file: %s", fileID)
	}

	// 2. Check if column exists
	values, ok := colsData.Values[columnName]
	if !ok {
		return nil, fmt.Errorf("column not found: %s", columnName)
	}

	// 3. Binary Search for Start Index
	startIdx := 0
	if timeFrom != nil {
		target := timeFrom.Unix()
		startIdx = sort.Search(len(colsData.Timestamps), func(i int) bool {
			return colsData.Timestamps[i] >= target
		})
	}

	// 4. Binary Search for End Index
	endIdx := len(colsData.Timestamps)
	if timeTo != nil {
		target := timeTo.Unix()
		idx := sort.Search(len(colsData.Timestamps), func(i int) bool {
			return colsData.Timestamps[i] > target
		})
		if idx < endIdx {
			endIdx = idx
		}
	}

	if startIdx >= endIdx {
		return []DataPoint{}, nil
	}

	// 5. Downsampling Logic
	totalPoints := endIdx - startIdx
	const maxPoints = 2000 // Target number of points for visualization

	if totalPoints <= maxPoints {
		// Return all points if within limit
		dataPoints := make([]DataPoint, 0, totalPoints)
		for i := startIdx; i < endIdx; i++ {
			val := values[i]
			if math.IsNaN(val) {
				continue
			}
			dataPoints = append(dataPoints, DataPoint{
				Timestamp: time.Unix(colsData.Timestamps[i], 0),
				Value:     val,
			})
		}
		return dataPoints, nil
	}

	// Simple specific-interval downsampling (Average pooling)
	// Group points into buckets and take the average
	dataPoints := make([]DataPoint, 0, maxPoints)
	bucketSize := float64(totalPoints) / float64(maxPoints)

	for i := 0; i < maxPoints; i++ {
		// Calculate bucket range
		pStart := startIdx + int(float64(i)*bucketSize)
		pEnd := startIdx + int(float64(i+1)*bucketSize)
		if pEnd > endIdx {
			pEnd = endIdx
		}
		if pStart >= pEnd {
			continue
		}

		var sum float64
		var count int
		// Use the timestamp of the first point in the bucket
		// or the middle one? First is simpler.
		ts := colsData.Timestamps[pStart]

		for j := pStart; j < pEnd; j++ {
			val := values[j]
			if !math.IsNaN(val) {
				sum += val
				count++
			}
		}

		if count > 0 {
			dataPoints = append(dataPoints, DataPoint{
				Timestamp: time.Unix(ts, 0),
				Value:     sum / float64(count),
			})
		}
	}

	return dataPoints, nil
}

// GetMetricColumns returns all metric columns excluding Timestamp.
func (s *CSVDataService) GetMetricColumns(fileID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.files[fileID]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", fileID)
	}

	metrics := make([]string, 0)
	for _, col := range file.Columns {
		if col != "Timestamp" {
			metrics = append(metrics, col)
		}
	}

	return metrics, nil
}

// DeleteFile removes a CSV file from the service.
func (s *CSVDataService) DeleteFile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.files[id]; !ok {
		return fmt.Errorf("file not found: %s", id)
	}

	delete(s.files, id)
	delete(s.columnData, id)

	return nil
}

// DeleteAll clears all data from memory.
func (s *CSVDataService) DeleteAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear maps
	s.files = make(map[string]*CSVFile)
	s.columnData = make(map[string]*ColumnData)
}

func parseTimestamp(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"02/01/2006 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
