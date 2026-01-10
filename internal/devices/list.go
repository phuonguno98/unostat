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

package devices

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/net"
)

// Dependency injection points for testing
var (
	diskPartitions = disk.Partitions
	diskUsage      = disk.Usage
	netInterfaces  = net.Interfaces
)

// DiskInfo represents disk device information.
type DiskInfo struct {
	Name       string
	Mountpoint string
	Filesystem string
	Total      uint64
}

// NetworkInfo represents network interface information.
type NetworkInfo struct {
	Name       string
	MacAddress string
	Addresses  []string
}

// ListDisks returns a list of available disk devices.
func ListDisks() ([]DiskInfo, error) {
	partitions, err := diskPartitions(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk partitions: %w", err)
	}

	disks := make([]DiskInfo, 0)
	seen := make(map[string]bool)

	for _, partition := range partitions {
		// Skip duplicate devices
		if seen[partition.Device] {
			continue
		}
		seen[partition.Device] = true

		usage, err := diskUsage(partition.Mountpoint)
		total := uint64(0)
		if err == nil {
			total = usage.Total
		}

		disks = append(disks, DiskInfo{
			Name:       partition.Device,
			Mountpoint: partition.Mountpoint,
			Filesystem: partition.Fstype,
			Total:      total,
		})
	}

	// Sort by device name
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Name < disks[j].Name
	})

	return disks, nil
}

// ListNetworkInterfaces returns a list of available network interfaces.
func ListNetworkInterfaces() ([]NetworkInfo, error) {
	interfaces, err := netInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	networks := make([]NetworkInfo, 0)

	for _, iface := range interfaces {
		// Skip interfaces without addresses
		if len(iface.Addrs) == 0 {
			continue
		}

		addresses := make([]string, 0, len(iface.Addrs))
		for _, addr := range iface.Addrs {
			addresses = append(addresses, addr.Addr)
		}

		networks = append(networks, NetworkInfo{
			Name:       iface.Name,
			MacAddress: iface.HardwareAddr,
			Addresses:  addresses,
		})
	}

	// Sort by interface name
	sort.Slice(networks, func(i, j int) bool {
		return networks[i].Name < networks[j].Name
	})

	return networks, nil
}

// FormatDisksTable formats disk information as a table.
func FormatDisksTable(disks []DiskInfo) string {
	var sb strings.Builder

	sb.WriteString("\nAvailable Disk Devices:\n")
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-30s %-20s %-15s %s\n", "DEVICE", "MOUNTPOINT", "FILESYSTEM", "SIZE"))
	sb.WriteString(strings.Repeat("-", 80))
	sb.WriteString("\n")

	for _, d := range disks {
		size := formatBytes(d.Total)
		sb.WriteString(fmt.Sprintf("%-30s %-20s %-15s %s\n",
			d.Name,
			truncate(d.Mountpoint, 20),
			d.Filesystem,
			size,
		))
	}

	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n")

	return sb.String()
}

// FormatNetworksTable formats network interface information as a table.
func FormatNetworksTable(networks []NetworkInfo) string {
	var sb strings.Builder

	sb.WriteString("\nAvailable Network Interfaces:\n")
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-40s %-17s %s\n", "INTERFACE", "MAC ADDRESS", "IP ADDRESSES"))
	sb.WriteString(strings.Repeat("-", 80))
	sb.WriteString("\n")

	for _, n := range networks {
		mac := n.MacAddress
		if mac == "" {
			mac = "N/A"
		}

		// Show first IP address on same line
		firstIP := "N/A"
		if len(n.Addresses) > 0 {
			firstIP = n.Addresses[0]
		}

		sb.WriteString(fmt.Sprintf("%-40s %-17s %s\n",
			n.Name,
			mac,
			firstIP,
		))

		// Show additional IPs on separate lines
		for i := 1; i < len(n.Addresses); i++ {
			sb.WriteString(fmt.Sprintf("%-40s %-17s %s\n", "", "", n.Addresses[i]))
		}
	}

	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n")

	return sb.String()
}

// formatBytes converts bytes to human-readable format.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
