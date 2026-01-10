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
	"time"

	"github.com/phuonguno98/unostat/pkg/metrics"
	"github.com/shirou/gopsutil/v3/net"
)

// NetworkCollector collects network bandwidth metrics.
type NetworkCollector struct {
	prevStats         map[string]metrics.NetworkIOStats
	includeInterfaces []string // Interfaces to monitor (empty = all)
	excludeInterfaces []string // Interfaces to exclude
	firstRun          bool
}

// NewNetworkCollector creates a new network collector instance.
// includeInterfaces: list of interface names to monitor (empty = all available)
// excludeInterfaces: list of interface names to exclude
func NewNetworkCollector(includeInterfaces, excludeInterfaces []string) *NetworkCollector {
	return &NetworkCollector{
		prevStats:         make(map[string]metrics.NetworkIOStats),
		includeInterfaces: includeInterfaces,
		excludeInterfaces: excludeInterfaces,
		firstRun:          true,
	}
}

// Collect gathers current network I/O metrics.
// Returns map of interface names to NetStats.
func (n *NetworkCollector) Collect() (map[string]metrics.NetStats, error) {
	ioCounters, err := net.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("failed to get network I/O counters: %w", err)
	}

	result := make(map[string]metrics.NetStats)
	now := time.Now()

	for _, counter := range ioCounters {
		interfaceName := counter.Name

		// Skip loopback interfaces
		if n.isLoopback(interfaceName) {
			continue
		}

		// Apply filters
		if !n.shouldMonitor(interfaceName) {
			continue
		}

		currentStats := metrics.NetworkIOStats{
			BytesSent: counter.BytesSent,
			BytesRecv: counter.BytesRecv,
			Timestamp: now,
		}

		// First run - just store baseline
		if n.firstRun {
			n.prevStats[interfaceName] = currentStats
			continue
		}

		// Check if we have previous stats for this interface
		prevStats, exists := n.prevStats[interfaceName]
		if !exists {
			n.prevStats[interfaceName] = currentStats
			continue
		}

		// Calculate bandwidth
		bandwidth := metrics.CalculateNetworkBandwidth(prevStats, currentStats)

		result[interfaceName] = metrics.NetStats{
			Bandwidth: bandwidth,
		}

		// Update previous stats
		n.prevStats[interfaceName] = currentStats
	}

	if n.firstRun {
		n.firstRun = false
		return nil, nil // Return nil on first run to indicate baseline collection
	}

	return result, nil
}

// isLoopback checks if an interface is a loopback interface.
func (n *NetworkCollector) isLoopback(interfaceName string) bool {
	// Common loopback interface names
	loopbacks := []string{"lo", "lo0", "Loopback"}
	for _, lo := range loopbacks {
		if interfaceName == lo {
			return true
		}
	}
	return false
}

// shouldMonitor checks if an interface should be monitored based on include/exclude filters.
func (n *NetworkCollector) shouldMonitor(interfaceName string) bool {
	// Check exclude list first
	if len(n.excludeInterfaces) > 0 {
		for _, excluded := range n.excludeInterfaces {
			if excluded == interfaceName {
				return false
			}
		}
	}

	// If include list is empty, monitor all (except excluded)
	if len(n.includeInterfaces) == 0 {
		return true
	}

	// Check include list
	for _, included := range n.includeInterfaces {
		if included == interfaceName {
			return true
		}
	}

	return false
}

// Name returns the collector name for logging purposes.
func (n *NetworkCollector) Name() string {
	return "Network"
}
