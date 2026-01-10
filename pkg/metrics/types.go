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

import "time"

// Snapshot represents a complete system metrics snapshot at a specific time.
type Snapshot struct {
	Timestamp time.Time
	CPU       float64              // CPU utilization percentage
	CPUWait   float64              // CPU iowait percentage (-1 if N/A)
	Memory    float64              // Memory utilization percentage
	Disks     map[string]DiskStats // Key: device name
	Networks  map[string]NetStats  // Key: interface name
}

// DiskStats represents disk I/O metrics for a single disk device.
type DiskStats struct {
	Utilization float64 // Percentage of time disk was busy
	Await       float64 // Average wait time for I/O operations in milliseconds
	IOPS        float64 // Input/Output Operations Per Second
}

// NetStats represents network metrics for a single interface.
type NetStats struct {
	Bandwidth float64 // Network bandwidth in bits per second
}

// CPUTimeStats represents CPU time statistics for delta calculations.
type CPUTimeStats struct {
	User      float64
	System    float64
	Idle      float64
	IOWait    float64
	Irq       float64
	SoftIrq   float64
	Steal     float64
	Guest     float64
	GuestNice float64
	Timestamp time.Time
}

// DiskIOStats represents disk I/O counters for delta calculations.
type DiskIOStats struct {
	ReadCount  uint64
	WriteCount uint64
	ReadTime   uint64 // Milliseconds
	WriteTime  uint64 // Milliseconds
	IOTime     uint64 // Milliseconds disk was busy
	Timestamp  time.Time
}

// NetworkIOStats represents network I/O counters for delta calculations.
type NetworkIOStats struct {
	BytesSent uint64
	BytesRecv uint64
	Timestamp time.Time
}
