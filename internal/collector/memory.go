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

package collector

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryCollector collects memory utilization metrics.
type MemoryCollector struct{}

// NewMemoryCollector creates a new memory collector instance.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Collect gathers current memory metrics.
// Returns memory utilization percentage.
func (m *MemoryCollector) Collect() (float64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("failed to get memory stats: %w", err)
	}

	// Calculate utilization: (Used / Total) × 100
	if vmStat.Total == 0 {
		return 0, fmt.Errorf("total memory is zero")
	}

	utilization := (float64(vmStat.Used) / float64(vmStat.Total)) * 100.0

	return utilization, nil
}

// Name returns the collector name for logging purposes.
func (m *MemoryCollector) Name() string {
	return "Memory"
}
