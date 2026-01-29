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
	"github.com/shirou/gopsutil/v3/disk"
)

// DiskCollector collects disk I/O metrics.
type DiskCollector struct {
	prevStats      map[string]metrics.DiskIOStats
	includeDevices []string // Devices to monitor (empty = all)
	excludeDevices []string // Devices to exclude
	firstRun       bool
}

// normalizeDeviceName strips /dev/ prefix from device names for consistent comparison.
// This allows users to specify devices as shown in list-devices (/dev/sdd)
// while internally matching against disk.IOCounters() format (sdd).
func normalizeDeviceName(name string) string {
	// Strip common prefixes
	if len(name) >= 5 && name[:5] == "/dev/" {
		return name[5:]
	}
	return name
}

// normalizeDeviceList normalizes all device names in a list.
func normalizeDeviceList(devices []string) []string {
	normalized := make([]string, len(devices))
	for i, device := range devices {
		normalized[i] = normalizeDeviceName(device)
	}
	return normalized
}

// NewDiskCollector creates a new disk collector instance.
// includeDevices: list of device names to monitor (empty = all available)
// excludeDevices: list of device names to exclude
// Device names can be specified with or without /dev/ prefix (e.g., "sdd" or "/dev/sdd")
func NewDiskCollector(includeDevices, excludeDevices []string) *DiskCollector {
	return &DiskCollector{
		prevStats:      make(map[string]metrics.DiskIOStats),
		includeDevices: normalizeDeviceList(includeDevices),
		excludeDevices: normalizeDeviceList(excludeDevices),
		firstRun:       true,
	}
}

// Collect gathers current disk I/O metrics.
// Returns map of device names to DiskStats.
func (d *DiskCollector) Collect() (map[string]metrics.DiskStats, error) {
	ioCounters, err := disk.IOCounters()
	if err != nil {
		return nil, fmt.Errorf("failed to get disk I/O counters: %w", err)
	}

	result := make(map[string]metrics.DiskStats)
	now := time.Now()

	for deviceName := range ioCounters {
		// Only copy the value when needed to avoid huge copy in loop
		// However, map iteration value is a copy anyway in Go ranges if we use value receiver
		// But here we can use pointer to map value or just use the value from map by key if needed,
		// but since we range over map, 'counter' is a copy.
		// To fix 'rangeValCopy', we can iterate keys only.
		counter := ioCounters[deviceName]

		// Apply filters
		if !d.shouldMonitor(deviceName) {
			continue
		}

		currentStats := metrics.DiskIOStats{
			ReadCount:  counter.ReadCount,
			WriteCount: counter.WriteCount,
			ReadTime:   counter.ReadTime,
			WriteTime:  counter.WriteTime,
			IOTime:     d.getIOTime(&counter),
			Timestamp:  now,
		}

		// First run - just store baseline
		if d.firstRun {
			d.prevStats[deviceName] = currentStats
			continue
		}

		// Check if we have previous stats for this device
		prevStats, exists := d.prevStats[deviceName]
		if !exists {
			d.prevStats[deviceName] = currentStats
			continue
		}

		// Calculate metrics
		utilization := metrics.CalculateDiskUtilization(prevStats, currentStats)
		await := metrics.CalculateDiskAwait(prevStats, currentStats)
		iops := metrics.CalculateDiskIOPS(prevStats, currentStats)

		result[deviceName] = metrics.DiskStats{
			Utilization: utilization,
			Await:       await,
			IOPS:        iops,
		}

		// Update previous stats
		d.prevStats[deviceName] = currentStats
	}

	if d.firstRun {
		d.firstRun = false
		return nil, nil // Return nil on first run to indicate baseline collection
	}

	return result, nil
}

// getIOTime extracts IOTime with platform-specific handling.
func (d *DiskCollector) getIOTime(counter *disk.IOCountersStat) uint64 {
	if runtime.GOOS == "windows" {
		// Windows: IoTime might not be available, use ReadTime + WriteTime as approximation
		if counter.IoTime == 0 {
			return counter.ReadTime + counter.WriteTime
		}
	}
	return counter.IoTime
}

// shouldMonitor checks if a device should be monitored based on include/exclude filters.
func (d *DiskCollector) shouldMonitor(deviceName string) bool {
	// Check exclude list first
	if len(d.excludeDevices) > 0 {
		for _, excluded := range d.excludeDevices {
			if excluded == deviceName {
				return false
			}
		}
	}

	// If include list is empty, monitor all (except excluded)
	if len(d.includeDevices) == 0 {
		return true
	}

	// Check include list
	for _, included := range d.includeDevices {
		if included == deviceName {
			return true
		}
	}

	return false
}

// Name returns the collector name for logging purposes.
func (d *DiskCollector) Name() string {
	return "Disk"
}
