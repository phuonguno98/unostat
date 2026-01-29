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
	"errors"
	"strings"
	"testing"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/net"
)

func TestListDisks(t *testing.T) {
	// Backup original functions
	origPartitions := diskPartitions
	origUsage := diskUsage
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	}()

	tests := []struct {
		name           string
		mockPartitions func(bool) ([]disk.PartitionStat, error)
		mockUsage      func(string) (*disk.UsageStat, error)
		wantCount      int
		wantErr        bool
	}{
		{
			name: "Success",
			mockPartitions: func(bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{
					{Device: "/dev/sda1", Mountpoint: "/", Fstype: "ext4"},
					{Device: "/dev/sdb1", Mountpoint: "/data", Fstype: "xfs"},
				}, nil
			},
			mockUsage: func(_ string) (*disk.UsageStat, error) {
				return &disk.UsageStat{Total: 1000}, nil
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "Partitions Error",
			mockPartitions: func(bool) ([]disk.PartitionStat, error) {
				return nil, errors.New("start failed")
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "Usage Error (Should proceed with 0 total)",
			mockPartitions: func(bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{
					{Device: "/dev/sda1", Mountpoint: "/", Fstype: "ext4"},
				}, nil
			},
			mockUsage: func(_ string) (*disk.UsageStat, error) {
				return nil, errors.New("usage failed")
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "Duplicate Devices",
			mockPartitions: func(bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{
					{Device: "/dev/sda1", Mountpoint: "/", Fstype: "ext4"},
					{Device: "/dev/sda1", Mountpoint: "/mnt", Fstype: "ext4"},
				}, nil
			},
			mockUsage: func(_ string) (*disk.UsageStat, error) {
				return &disk.UsageStat{Total: 1000}, nil
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diskPartitions = tt.mockPartitions
			diskUsage = tt.mockUsage

			got, err := ListDisks()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListDisks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ListDisks() count = %d, want %d", len(got), tt.wantCount)
			}
			if tt.name == "Usage Error (Should proceed with 0 total)" && len(got) > 0 {
				if got[0].Total != 0 {
					t.Errorf("Expected Total=0 for usage error, got %d", got[0].Total)
				}
			}
		})
	}
}

func TestListNetworkInterfaces(t *testing.T) {
	origInterfaces := netInterfaces
	defer func() { netInterfaces = origInterfaces }()

	tests := []struct {
		name           string
		mockInterfaces func() (net.InterfaceStatList, error)
		wantCount      int
		wantErr        bool
	}{
		{
			name: "Success",
			mockInterfaces: func() (net.InterfaceStatList, error) {
				return net.InterfaceStatList{
					{Name: "eth0", Addrs: []net.InterfaceAddr{{Addr: "192.168.1.1"}}},
					{Name: "eth1", Addrs: []net.InterfaceAddr{{Addr: "10.0.0.1"}}},
				}, nil
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "Error",
			mockInterfaces: func() (net.InterfaceStatList, error) {
				return nil, errors.New("net failed")
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "Skip No Addresses",
			mockInterfaces: func() (net.InterfaceStatList, error) {
				return net.InterfaceStatList{
					{Name: "eth0", Addrs: nil},
					{Name: "eth1", Addrs: []net.InterfaceAddr{{Addr: "10.0.0.1"}}},
				}, nil
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netInterfaces = tt.mockInterfaces
			got, err := ListNetworkInterfaces()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListNetworkInterfaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ListNetworkInterfaces() count = %d, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestFormatDisksTable(t *testing.T) {
	disks := []DiskInfo{
		{
			Name:       "disk1",
			Mountpoint: "/mnt/data",
			Filesystem: "ext4",
			Total:      1024 * 1024 * 1024 * 100, // 100 GB
		},
		{
			Name:       "disk2",
			Mountpoint: "/very/long/path/name/that/exceeds/limit",
			Filesystem: "ntfs",
			Total:      0,
		},
	}

	out := FormatDisksTable(disks)
	if out == "" {
		t.Error("Output is empty")
	}
	// Check for content presence
	if !strings.Contains(out, "disk1") {
		t.Error("Missing disk1")
	}
	if !strings.Contains(out, "100.0 GB") {
		t.Error("Missing size formatting")
	}
	if !strings.Contains(out, "...") {
		t.Error("Missing truncation")
	}
}

func TestFormatNetworksTable(t *testing.T) {
	networks := []NetworkInfo{
		{
			Name:       "eth0",
			MacAddress: "AA:BB:CC:DD:EE:FF",
			Addresses:  []string{"192.168.1.1", "fe80::1"},
		},
		{
			Name:       "lo",
			MacAddress: "",
			Addresses:  []string{},
		},
	}

	out := FormatNetworksTable(networks)
	if out == "" {
		t.Error("Output is empty")
	}

	if !strings.Contains(out, "eth0") {
		t.Error("Missing eth0")
	}
	if !strings.Contains(out, "AA:BB:CC:DD:EE:FF") {
		t.Error("Missing MAC")
	}
	if !strings.Contains(out, "192.168.1.1") {
		t.Error("Missing first IP")
	}
	if !strings.Contains(out, "fe80::1") {
		t.Error("Missing second IP")
	}
	if !strings.Contains(out, "N/A") {
		t.Error("Missing N/A for empty MAC/Addresses")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024 * 5, "5.0 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"Short", 10, "Short"},
		{"ExactLength", 11, "ExactLength"},
		{"TooLongString", 10, "TooLong..."},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
