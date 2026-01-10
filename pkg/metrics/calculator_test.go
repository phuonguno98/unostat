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

package metrics

import (
	"math"
	"testing"
	"time"
)

func TestCalculateCPUUtilization(t *testing.T) {
	tests := []struct {
		name     string
		prev     CPUTimeStats
		current  CPUTimeStats
		expected float64
	}{
		{
			name: "Normal usage",
			prev: CPUTimeStats{
				User: 100, System: 50, Idle: 800, IOWait: 10,
				Timestamp: time.Now(),
			},
			current: CPUTimeStats{
				User: 110, System: 60, Idle: 810, IOWait: 15, // Deltas: U:10, S:10, I:10, IO:5 -> Total: 35
				Timestamp: time.Now().Add(1 * time.Second),
			},
			// Total Delta = 10 (User) + 10 (System) + 10 (Idle) + 5 (IO) = 35
			// Idle Delta = 10
			// Util = 100 * (1 - 10/35) = 100 * (25/35) = 100 * 0.7142857 = 71.42857
			expected: 71.42857142857143,
		},
		{
			name: "Zero timestamp (First run)",
			prev: CPUTimeStats{}, // Zero timestamp
			current: CPUTimeStats{
				User:      100,
				Timestamp: time.Now(),
			},
			expected: 0.0,
		},
		{
			name: "No change (Zero delta total)",
			prev: CPUTimeStats{
				User: 100, Idle: 100,
				Timestamp: time.Now(),
			},
			current: CPUTimeStats{
				User: 100, Idle: 100,
				Timestamp: time.Now().Add(1 * time.Second),
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCPUUtilization(&tt.prev, &tt.current)
			if math.Abs(got-tt.expected) > 0.00001 {
				t.Errorf("CalculateCPUUtilization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateCPUIOWait(t *testing.T) {
	tests := []struct {
		name     string
		prev     CPUTimeStats
		current  CPUTimeStats
		expected float64
	}{
		{
			name: "Normal IOWait",
			prev: CPUTimeStats{
				User: 100, System: 50, Idle: 800, IOWait: 10,
				Timestamp: time.Now(),
			},
			current: CPUTimeStats{
				User: 110, System: 60, Idle: 810, IOWait: 20, // Delta IO: 10
				Timestamp: time.Now().Add(1 * time.Second), // Delta Total: 40 (U:10, S:10, I:10, IO:10)
			},
			// Util = 100 * (10 / 40) = 25.0
			expected: 25.0,
		},
		{
			name: "Unavailable IOWait (Negative)",
			prev: CPUTimeStats{Timestamp: time.Now()},
			current: CPUTimeStats{
				IOWait:    -1,
				Timestamp: time.Now().Add(1 * time.Second),
			},
			expected: -1.0,
		},
		{
			name:     "First run",
			prev:     CPUTimeStats{},
			current:  CPUTimeStats{},
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCPUIOWait(&tt.prev, &tt.current)
			if math.Abs(got-tt.expected) > 0.00001 {
				t.Errorf("CalculateCPUIOWait() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateDiskUtilization(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		prev     DiskIOStats
		current  DiskIOStats
		expected float64
	}{
		{
			name: "50% Utilization",
			prev: DiskIOStats{
				IOTime:    1000,
				Timestamp: now,
			},
			current: DiskIOStats{
				IOTime:    1500,                     // Delta 500ms
				Timestamp: now.Add(1 * time.Second), // Delta 1000ms
			},
			expected: 50.0,
		},
		{
			name: "Over 100% Cap",
			prev: DiskIOStats{
				IOTime:    1000,
				Timestamp: now,
			},
			current: DiskIOStats{
				IOTime:    2500,                     // Delta 1500ms
				Timestamp: now.Add(1 * time.Second), // Delta 1000ms
			},
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDiskUtilization(tt.prev, tt.current)
			if math.Abs(got-tt.expected) > 0.00001 {
				t.Errorf("CalculateDiskUtilization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateDiskAwait(t *testing.T) {
	tests := []struct {
		name     string
		prev     DiskIOStats
		current  DiskIOStats
		expected float64
	}{
		{
			name: "Normal Await",
			prev: DiskIOStats{
				ReadCount: 10, WriteCount: 10,
				ReadTime: 100, WriteTime: 100,
				Timestamp: time.Now(),
			},
			current: DiskIOStats{
				ReadCount: 15, WriteCount: 15, // Delta Ops: 5+5 = 10
				ReadTime: 150, WriteTime: 150, // Delta Time: 50+50 = 100
				Timestamp: time.Now().Add(1 * time.Second),
			},
			// Await = 100ms / 10 ops = 10ms
			expected: 10.0,
		},
		{
			name: "Zero Ops",
			prev: DiskIOStats{
				ReadCount: 10,
				Timestamp: time.Now(),
			},
			current: DiskIOStats{
				ReadCount: 10, // Delta 0
				Timestamp: time.Now().Add(1 * time.Second),
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDiskAwait(tt.prev, tt.current)
			if math.Abs(got-tt.expected) > 0.00001 {
				t.Errorf("CalculateDiskAwait() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateNetworkBandwidth(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		prev     NetworkIOStats
		current  NetworkIOStats
		expected float64
	}{
		{
			name: "1 KBps",
			prev: NetworkIOStats{
				BytesSent: 1000,
				BytesRecv: 1000,
				Timestamp: now,
			},
			current: NetworkIOStats{
				BytesSent: 2000, // Delta 1000
				BytesRecv: 2024, // Delta 1024
				Timestamp: now.Add(1 * time.Second),
			},
			// Total Bytes Delta: 2024
			// Bits: 2024 * 8 = 16192
			// Time: 1s
			expected: 16192.0,
		},
		{
			name: "Zero Time Delta",
			prev: NetworkIOStats{
				Timestamp: now,
			},
			current: NetworkIOStats{
				Timestamp: now,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateNetworkBandwidth(tt.prev, tt.current)
			if math.Abs(got-tt.expected) > 0.00001 {
				t.Errorf("CalculateNetworkBandwidth() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateDiskIOPS(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		prev     DiskIOStats
		current  DiskIOStats
		expected float64
	}{
		{
			name: "100 IOPS",
			prev: DiskIOStats{
				ReadCount: 1000, WriteCount: 1000,
				Timestamp: now,
			},
			current: DiskIOStats{
				ReadCount: 1050, WriteCount: 1050, // Delta 50+50=100
				Timestamp: now.Add(1 * time.Second),
			},
			expected: 100.0,
		},
		{
			name:     "Zero Timestamp",
			prev:     DiskIOStats{},
			current:  DiskIOStats{Timestamp: now},
			expected: 0.0,
		},
		{
			name: "Zero Time Delta",
			prev: DiskIOStats{
				ReadCount: 10,
				Timestamp: now,
			},
			current: DiskIOStats{
				ReadCount: 20,
				Timestamp: now,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDiskIOPS(tt.prev, tt.current)
			if math.Abs(got-tt.expected) > 0.00001 {
				t.Errorf("CalculateDiskIOPS() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateEdgeCases(t *testing.T) {
	// Test IsZero timestamp checks
	emptyCPU := CPUTimeStats{}
	validCPU := CPUTimeStats{Timestamp: time.Now()}

	if val := CalculateCPUUtilization(&emptyCPU, &validCPU); val != 0.0 {
		t.Errorf("CalculateCPUUtilization(empty, valid) = %v, want 0.0", val)
	}

	if val := CalculateCPUIOWait(&emptyCPU, &validCPU); val != -1.0 {
		t.Errorf("CalculateCPUIOWait(empty, valid) = %v, want -1.0", val)
	}

	// Test deltaTotal <= 0 for CPU
	cpu1 := CPUTimeStats{User: 100, Timestamp: time.Now()}
	cpu2 := CPUTimeStats{User: 100, Timestamp: time.Now().Add(time.Second)} // Same values -> delta 0

	if val := CalculateCPUIOWait(&cpu1, &cpu2); val != 0.0 {
		t.Errorf("CalculateCPUIOWait(delta=0) = %v, want 0.0", val)
	}

	// Test IsZero for Disk Utils
	emptyDisk := DiskIOStats{}
	validDisk := DiskIOStats{Timestamp: time.Now()}
	if val := CalculateDiskUtilization(emptyDisk, validDisk); val != 0.0 {
		t.Errorf("CalculateDiskUtilization(empty) = %v, want 0.0", val)
	}

	if val := CalculateDiskAwait(emptyDisk, validDisk); val != 0.0 {
		t.Errorf("CalculateDiskAwait(empty) = %v, want 0.0", val)
	}

	// Test IsZero for Network
	emptyNet := NetworkIOStats{}
	validNet := NetworkIOStats{Timestamp: time.Now()}
	if val := CalculateNetworkBandwidth(emptyNet, validNet); val != 0.0 {
		t.Errorf("CalculateNetworkBandwidth(empty) = %v, want 0.0", val)
	}
}
