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
	"runtime"
	"time"

	"github.com/phuonguno98/unostat/pkg/metrics"
	"github.com/shirou/gopsutil/v3/cpu"
)

// CPUCollector collects CPU utilization and iowait metrics.
type CPUCollector struct {
	prevStats metrics.CPUTimeStats
	firstRun  bool
}

// NewCPUCollector creates a new CPU collector instance.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{
		firstRun: true,
	}
}

// Collect gathers current CPU metrics and calculates utilization.
// Returns CPU utilization percentage and iowait percentage.
// IOWait returns -1.0 if not available on the platform.
func (c *CPUCollector) Collect() (utilization, iowait float64, err error) {
	currentStats, err := c.getCPUTimeStats()
	if err != nil {
		return 0, -1.0, fmt.Errorf("failed to get CPU stats: %w", err)
	}

	// First run - just store baseline
	if c.firstRun {
		c.prevStats = currentStats
		c.firstRun = false
		return 0, -1.0, nil
	}

	// Calculate metrics
	utilization = metrics.CalculateCPUUtilization(&c.prevStats, &currentStats)
	iowait = metrics.CalculateCPUIOWait(&c.prevStats, &currentStats)

	// Update previous stats
	c.prevStats = currentStats

	return utilization, iowait, nil
}

// getCPUTimeStats retrieves CPU time statistics from the system.
func (c *CPUCollector) getCPUTimeStats() (metrics.CPUTimeStats, error) {
	stats := metrics.CPUTimeStats{
		Timestamp: time.Now(),
	}

	// Get CPU times (aggregated across all CPUs)
	times, err := cpu.Times(false)
	if err != nil {
		return stats, err
	}

	if len(times) == 0 {
		return stats, fmt.Errorf("no CPU time stats available")
	}

	t := times[0]

	stats.User = t.User
	stats.System = t.System
	stats.Idle = t.Idle
	stats.Irq = t.Irq
	stats.SoftIrq = t.Softirq
	stats.Steal = t.Steal
	stats.Guest = t.Guest
	stats.GuestNice = t.GuestNice

	// IOWait handling per platform
	stats.IOWait = c.getIOWait(&t)

	return stats, nil
}

// getIOWait extracts iowait value with platform-specific handling.
func (c *CPUCollector) getIOWait(t *cpu.TimesStat) float64 {
	switch runtime.GOOS {
	case "windows":
		// Windows doesn't have iowait concept
		return -1.0
	case "darwin":
		// macOS has limited iowait support
		// gopsutil may return 0 or a very rough estimate
		if t.Iowait == 0 {
			return -1.0
		}
		return t.Iowait
	case "linux":
		// Linux has accurate iowait metrics
		return t.Iowait
	default:
		// Unknown platform
		return -1.0
	}
}

// Name returns the collector name for logging purposes.
func (c *CPUCollector) Name() string {
	return "CPU"
}
