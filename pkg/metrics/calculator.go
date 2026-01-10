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

// CalculateCPUUtilization calculates CPU utilization percentage from two CPU time snapshots.
// Formula: 100 * (1 - ΔIdle / ΔTotal)
func CalculateCPUUtilization(prev, current *CPUTimeStats) float64 {
	if prev.Timestamp.IsZero() {
		return 0.0
	}

	prevTotal := prev.User + prev.System + prev.Idle + prev.IOWait + prev.Irq + prev.SoftIrq + prev.Steal
	currentTotal := current.User + current.System + current.Idle + current.IOWait + current.Irq + current.SoftIrq + current.Steal

	deltaTotal := currentTotal - prevTotal
	deltaIdle := current.Idle - prev.Idle

	if deltaTotal <= 0 {
		return 0.0
	}

	return 100.0 * (1.0 - deltaIdle/deltaTotal)
}

// CalculateCPUIOWait calculates CPU iowait percentage from two CPU time snapshots.
// Formula: 100 * (ΔIOWait / ΔTotal)
// Returns -1.0 if iowait is not available on the platform.
func CalculateCPUIOWait(prev, current *CPUTimeStats) float64 {
	if prev.Timestamp.IsZero() {
		return -1.0
	}

	// If current IOWait is negative, it means the platform doesn't support it
	if current.IOWait < 0 {
		return -1.0
	}

	prevTotal := prev.User + prev.System + prev.Idle + prev.IOWait + prev.Irq + prev.SoftIrq + prev.Steal
	currentTotal := current.User + current.System + current.Idle + current.IOWait + current.Irq + current.SoftIrq + current.Steal

	deltaTotal := currentTotal - prevTotal
	deltaIOWait := current.IOWait - prev.IOWait

	if deltaTotal <= 0 {
		return 0.0
	}

	return 100.0 * (deltaIOWait / deltaTotal)
}

// CalculateDiskUtilization calculates disk utilization percentage from two I/O snapshots.
// Formula: (ΔIOTime / Δt) × 100
func CalculateDiskUtilization(prev, current DiskIOStats) float64 {
	if prev.Timestamp.IsZero() {
		return 0.0
	}

	deltaTime := current.Timestamp.Sub(prev.Timestamp).Milliseconds()
	if deltaTime <= 0 {
		return 0.0
	}

	deltaIOTime := float64(current.IOTime - prev.IOTime)
	utilization := (deltaIOTime / float64(deltaTime)) * 100.0

	// Cap at 100% (can exceed due to rounding or multiple queues)
	if utilization > 100.0 {
		utilization = 100.0
	}

	return utilization
}

// CalculateDiskAwait calculates average I/O wait time in milliseconds.
// Formula: Δ(ReadTime + WriteTime) / Δ(ReadCount + WriteCount)
func CalculateDiskAwait(prev, current DiskIOStats) float64 {
	if prev.Timestamp.IsZero() {
		return 0.0
	}

	deltaReadCount := current.ReadCount - prev.ReadCount
	deltaWriteCount := current.WriteCount - prev.WriteCount
	totalOps := deltaReadCount + deltaWriteCount

	if totalOps == 0 {
		return 0.0
	}

	deltaReadTime := current.ReadTime - prev.ReadTime
	deltaWriteTime := current.WriteTime - prev.WriteTime
	totalTime := deltaReadTime + deltaWriteTime

	return float64(totalTime) / float64(totalOps)
}

// CalculateDiskIOPS calculates the IOPS (Input/Output Operations Per Second).
// Formula: (ΔReadCount + ΔWriteCount) / Δt
func CalculateDiskIOPS(prev, current DiskIOStats) float64 {
	if prev.Timestamp.IsZero() {
		return 0.0
	}

	deltaTime := current.Timestamp.Sub(prev.Timestamp).Seconds()
	if deltaTime <= 0 {
		return 0.0
	}

	deltaReadCount := current.ReadCount - prev.ReadCount
	deltaWriteCount := current.WriteCount - prev.WriteCount
	totalOps := deltaReadCount + deltaWriteCount

	return float64(totalOps) / deltaTime
}

// CalculateNetworkBandwidth calculates network bandwidth in bits per second.
// Formula: [Δ(BytesSent + BytesRecv) × 8] / Δt
func CalculateNetworkBandwidth(prev, current NetworkIOStats) float64 {
	if prev.Timestamp.IsZero() {
		return 0.0
	}

	deltaTime := current.Timestamp.Sub(prev.Timestamp).Seconds()
	if deltaTime <= 0 {
		return 0.0
	}

	deltaSent := current.BytesSent - prev.BytesSent
	deltaRecv := current.BytesRecv - prev.BytesRecv
	totalBytes := deltaSent + deltaRecv

	// Convert bytes to bits and divide by time
	return float64(totalBytes*8) / deltaTime
}
