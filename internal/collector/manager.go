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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/phuonguno98/unostat/internal/config"
	"github.com/phuonguno98/unostat/pkg/metrics"
)

var startUpDelay = 1 * time.Second

// Manager orchestrates all metric collectors.
type Manager struct {
	config      *config.Config
	cpu         *CPUCollector
	memory      *MemoryCollector
	disk        *DiskCollector
	network     *NetworkCollector
	metricsChan chan<- *metrics.Snapshot
	ticker      *time.Ticker
	logger      *slog.Logger
}

// NewManager creates a new collector manager instance.
func NewManager(cfg *config.Config, metricsChan chan<- *metrics.Snapshot, logger *slog.Logger) *Manager {
	return &Manager{
		config:      cfg,
		cpu:         NewCPUCollector(),
		memory:      NewMemoryCollector(),
		disk:        NewDiskCollector(cfg.IncludeDisks, cfg.ExcludeDisks),
		network:     NewNetworkCollector(cfg.IncludeNetworks, cfg.ExcludeNetworks),
		metricsChan: metricsChan,
		logger:      logger,
	}
}

// Start begins the collection loop.
// It performs an initial baseline collection, then collects metrics at the configured interval.
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting collector manager",
		"interval", m.config.SamplingInterval,
	)

	// Perform baseline collection
	m.logger.Info("Performing baseline collection...")
	if err := m.collectOnce(); err != nil {
		m.logger.Warn("Baseline collection had errors", "error", err)
	}

	// Wait a bit before starting regular collection
	select {
	case <-time.After(startUpDelay):
	case <-ctx.Done():
		return nil
	}

	// Start ticker for regular collection
	m.ticker = time.NewTicker(m.config.SamplingInterval)
	defer m.ticker.Stop()

	m.logger.Info("Collector manager started")

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Collector manager stopping...")
			return nil

		case <-m.ticker.C:
			if err := m.collectOnce(); err != nil {
				m.logger.Error("Collection failed", "error", err)
			}
		}
	}
}

// collectOnce performs a single collection cycle concurrently.
// It gathers metrics from all collectors in parallel to minimize total collection time.
func (m *Manager) collectOnce() error {
	snapshot := &metrics.Snapshot{
		Timestamp: time.Now(),
		Disks:     make(map[string]metrics.DiskStats),
		Networks:  make(map[string]metrics.NetStats),
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex // Protects snapshot updates
	)

	// We have 4 collectors to run in parallel
	wg.Add(4)

	// Collect CPU metrics
	go func() {
		defer wg.Done()
		cpuUtil, cpuWait, err := m.cpu.Collect()
		if err != nil {
			m.logger.Warn("Failed to collect CPU metrics", "error", err)
		} else {
			mu.Lock()
			snapshot.CPU = cpuUtil
			snapshot.CPUWait = cpuWait
			mu.Unlock()
		}
	}()

	// Collect Memory metrics
	go func() {
		defer wg.Done()
		memUtil, err := m.memory.Collect()
		if err != nil {
			m.logger.Warn("Failed to collect memory metrics", "error", err)
		} else {
			mu.Lock()
			snapshot.Memory = memUtil
			mu.Unlock()
		}
	}()

	// Collect Disk metrics
	go func() {
		defer wg.Done()
		diskStats, err := m.disk.Collect()
		if err != nil {
			m.logger.Warn("Failed to collect disk metrics", "error", err)
		} else if diskStats != nil {
			mu.Lock()
			snapshot.Disks = diskStats
			mu.Unlock()
		}
	}()

	// Collect Network metrics
	go func() {
		defer wg.Done()
		netStats, err := m.network.Collect()
		if err != nil {
			m.logger.Warn("Failed to collect network metrics", "error", err)
		} else if netStats != nil {
			mu.Lock()
			snapshot.Networks = netStats
			mu.Unlock()
		}
	}()

	// Wait for all collectors to finish
	wg.Wait()

	// Check if this is baseline collection (or no useful data)
	mu.Lock()
	disksEmpty := len(snapshot.Disks) == 0
	netsEmpty := len(snapshot.Networks) == 0
	mu.Unlock()

	if disksEmpty || netsEmpty {
		m.logger.Debug("Baseline collection completed")
		return nil
	}

	// Send snapshot to channel (non-blocking)
	select {
	case m.metricsChan <- snapshot:
		m.logger.Debug("Snapshot sent",
			"cpu", snapshot.CPU,
			"cpu_wait", snapshot.CPUWait,
			"memory", snapshot.Memory,
			"disks", len(snapshot.Disks),
			"networks", len(snapshot.Networks),
		)
	default:
		m.logger.Warn("Metrics channel full, dropping snapshot")
		return fmt.Errorf("metrics channel full")
	}

	return nil
}

// Stop gracefully stops the collector manager.
func (m *Manager) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.logger.Info("Collector manager stopped")
}
