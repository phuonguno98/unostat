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
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/phuonguno98/unostat/internal/config"
	"github.com/phuonguno98/unostat/pkg/metrics"
)

func TestMemoryCollector(t *testing.T) {
	c := NewMemoryCollector()
	util, err := c.Collect()
	if err != nil {
		t.Fatalf("MemoryCollector.Collect() error = %v", err)
	}
	if util < 0 || util > 100 {
		t.Errorf("Memory utilization = %v, want [0, 100]", util)
	}
	if c.Name() != "Memory" {
		t.Errorf("Name() = %v, want Memory", c.Name())
	}
}

func TestCPUCollector(t *testing.T) {
	c := NewCPUCollector()

	// First run (baseline)
	util, _, err := c.Collect()
	if err != nil {
		t.Fatalf("First Collect() error = %v", err)
	}
	// Initial utilization should be 0 because it's first run
	if util != 0 {
		t.Logf("First run util = %v (expected 0/undefined baseline)", util)
	}

	// Sleep to allow some CPU time change
	time.Sleep(100 * time.Millisecond)

	// Second run (should have valid delta)
	var iowait float64
	util, iowait, err = c.Collect()
	if err != nil {
		t.Fatalf("Second Collect() error = %v", err)
	}

	if util < 0 || util > 100 {
		t.Errorf("CPU utilization = %v, want [0, 100]", util)
	}
	// IOWait can be -1 if not supported
	if iowait != -1.0 && (iowait < 0 || iowait > 100) {
		t.Errorf("CPU iowait = %v, want [0, 100] or -1", iowait)
	}
}

func TestDiskCollector(t *testing.T) {
	c := NewDiskCollector(nil, nil)

	// First run
	stats, err := c.Collect()
	if err != nil {
		t.Fatalf("First Collect() error = %v", err)
	}
	if stats != nil {
		t.Error("First Collect() should return nil stats (baseline)")
	}

	time.Sleep(100 * time.Millisecond)

	// Second run
	stats, err = c.Collect()
	if err != nil {
		t.Fatalf("Second Collect() error = %v", err)
	}
	if stats == nil {
		t.Fatal("Second Collect() returned nil stats")
	}

	for name, stat := range stats {
		if stat.Utilization < 0 { // Can exceed 100 technically
			t.Errorf("Disk %s Util = %v, want >= 0", name, stat.Utilization)
		}
	}

	if c.Name() != "Disk" {
		t.Errorf("Name() = %v, want Disk", c.Name())
	}
}

func TestNetworkCollector(t *testing.T) {
	c := NewNetworkCollector(nil, nil)

	// First run
	stats, err := c.Collect()
	if err != nil {
		t.Fatalf("First Collect() error = %v", err)
	}
	if stats != nil {
		t.Error("First Collect() should return nil stats (baseline)")
	}

	time.Sleep(100 * time.Millisecond)

	// Second run
	stats, err = c.Collect()
	if err != nil {
		t.Fatalf("Second Collect() error = %v", err)
	}

	// Network stats might be empty if no traffic or interfaces found/filtered
	// But it shouldn't error.
	for name, stat := range stats {
		if stat.Bandwidth < 0 {
			t.Errorf("Net %s BW = %v, want >= 0", name, stat.Bandwidth)
		}
	}

	if c.Name() != "Network" {
		t.Errorf("Name() = %v, want Network", c.Name())
	}
}

func TestManager_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metricsChan := make(chan *metrics.Snapshot, 10)
	cfg := &config.Config{
		SamplingInterval: 100 * time.Millisecond,
		IncludeDisks:     nil,
		IncludeNetworks:  nil,
	}

	mgr := NewManager(cfg, metricsChan, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Run manager in background
	errChan := make(chan error)
	go func() {
		errChan <- mgr.Start(ctx)
	}()

	// Consume metrics if any produced
	go func() {
		for range metricsChan {
			// Drain
		}
	}()

	// Wait for completion (context timeout)
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Manager.Start() returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Manager failed to stop within timeout")
	}
}

func TestDiskCollector_ShouldMonitor(t *testing.T) {
	tests := []struct {
		name    string
		include []string
		exclude []string
		device  string
		want    bool
	}{
		{
			name:    "Default (Monitor All)",
			include: nil,
			exclude: nil,
			device:  "sda",
			want:    true,
		},
		{
			name:    "Exclude Specific",
			include: nil,
			exclude: []string{"sda"},
			device:  "sda",
			want:    false,
		},
		{
			name:    "Exclude Different",
			include: nil,
			exclude: []string{"sdb"},
			device:  "sda",
			want:    true,
		},
		{
			name:    "Include Specific (Match)",
			include: []string{"sda"},
			exclude: nil,
			device:  "sda",
			want:    true,
		},
		{
			name:    "Include Specific (No Match)",
			include: []string{"sda"},
			exclude: nil,
			device:  "sdb",
			want:    false,
		},
		{
			name:    "Exclude Overrides Include",
			include: []string{"sda"},
			exclude: []string{"sda"},
			device:  "sda",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDiskCollector(tt.include, tt.exclude)
			if got := c.shouldMonitor(tt.device); got != tt.want {
				t.Errorf("shouldMonitor(%q) = %v, want %v", tt.device, got, tt.want)
			}
		})
	}
}

func TestNetworkCollector_ShouldMonitor(t *testing.T) {
	tests := []struct {
		name    string
		include []string
		exclude []string
		iface   string
		want    bool
	}{
		{"Default", nil, nil, "eth0", true},
		{"Exclude", nil, []string{"eth0"}, "eth0", false},
		{"Include Match", []string{"eth0"}, nil, "eth0", true},
		{"Include No Match", []string{"eth0"}, nil, "eth1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewNetworkCollector(tt.include, tt.exclude)
			if got := c.shouldMonitor(tt.iface); got != tt.want {
				t.Errorf("shouldMonitor(%q) = %v, want %v", tt.iface, got, tt.want)
			}
		})
	}
}

func TestManager_Lifecycle(t *testing.T) {
	// Save/Restore original delay
	origDelay := startUpDelay
	startUpDelay = 10 * time.Millisecond
	defer func() { startUpDelay = origDelay }()

	cfg := &config.Config{
		SamplingInterval: 50 * time.Millisecond,
	}
	ch := make(chan *metrics.Snapshot, 10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewManager(cfg, ch, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Start in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- m.Start(ctx)
	}()

	// Wait for at least one metric or timeout
	select {
	case <-ch:
		// Received metrics!
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for metrics")
	case err := <-errChan:
		t.Fatalf("Start exited early: %v", err)
	}

	// Test manual Stop (coverage)
	m.Stop()

	// Cancel context to stop Start loop correctly
	cancel()

	// Wait for Start to return
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Start did not return after cancellation")
	}
}
