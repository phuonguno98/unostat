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

package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/phuonguno98/unostat/internal/collector"
	"github.com/phuonguno98/unostat/internal/config"
	"github.com/phuonguno98/unostat/internal/exporter"
	"github.com/phuonguno98/unostat/pkg/metrics"
	"github.com/phuonguno98/unostat/pkg/version"
	"github.com/spf13/cobra"
)

var (
	// Collect command specific flags
	samplingInterval time.Duration
	outputPath       string
	bufferSize       int
	flushInterval    time.Duration
	includeDisks     string
	excludeDisks     string
	includeNetworks  string
	excludeNetworks  string
)

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Start UnoStat system monitoring",
	Long: `Start UnoStat to monitor system performance metrics (CPU, RAM, Disk, Network).
Data is collected and saved to CSV files.

Examples:
  # Run in foreground with default settings
  unostat collect

  # Custom interval and filters
  unostat collect --interval 5s --include-disks "C:"`,
	RunE: runCollect,
}

func init() {
	rootCmd.AddCommand(collectCmd)

	// Define flags specifically for collect command
	collectCmd.Flags().DurationVar(&samplingInterval, "interval", config.DefaultSamplingInterval,
		"Sampling interval (e.g., 1s, 30s, 1m)")
	collectCmd.Flags().StringVarP(&outputPath, "output", "o", "",
		"Output CSV file path (default: <hostname>_<timestamp>.csv)")
	collectCmd.Flags().IntVar(&bufferSize, "buffer-size", config.DefaultBufferSize,
		"Buffer size for CSV writer")
	collectCmd.Flags().DurationVar(&flushInterval, "flush-interval", config.DefaultFlushInterval,
		"Flush interval for CSV writer")

	// Filter flags
	collectCmd.Flags().StringVar(&includeDisks, "include-disks", "",
		"Comma-separated list of disk devices to monitor (empty = all)")
	collectCmd.Flags().StringVar(&excludeDisks, "exclude-disks", "",
		"Comma-separated list of disk devices to exclude")
	collectCmd.Flags().StringVar(&includeNetworks, "include-networks", "",
		"Comma-separated list of network interfaces to monitor (empty = all)")
	collectCmd.Flags().StringVar(&excludeNetworks, "exclude-networks", "",
		"Comma-separated list of network interfaces to exclude")
}

// buildConfig creates a Config object from parsed flags.
func buildConfig() (*config.Config, error) {
	cfg := &config.Config{
		SamplingInterval: samplingInterval,
		OutputPath:       outputPath,
		BufferSize:       bufferSize,
		FlushInterval:    flushInterval,
		LogLevel:         logLevel, // Access global var from root.go
		LogFile:          logFile,  // Access global var from root.go
		Timezone:         timezone, // Access global var from root.go
	}

	// Set defaults if not specified
	if cfg.OutputPath == "" {
		cfg.OutputPath = config.GetDefaultOutputPath()
	}

	// Parse filter lists
	cfg.IncludeDisks = config.ParseCommaSeparated(includeDisks)
	cfg.ExcludeDisks = config.ParseCommaSeparated(excludeDisks)
	cfg.IncludeNetworks = config.ParseCommaSeparated(includeNetworks)
	cfg.ExcludeNetworks = config.ParseCommaSeparated(excludeNetworks)

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// runCollect is the main monitoring entry point.
func runCollect(cmd *cobra.Command, args []string) error {
	// Build configuration from flags
	var err error
	cfg, err = buildConfig()
	if err != nil {
		return err
	}

	// Initialize logger
	logger := initLogger(cfg)

	logger.Info("Starting UnoStat",
		"version", version.Info(),
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
	)
	logger.Info("Configuration loaded", "config", cfg.String())

	// Check platform capabilities
	checkPlatformCapabilities(logger)

	// Create metrics channel (buffered to avoid blocking collectors)
	metricsChan := make(chan *metrics.Snapshot, 10)

	// Create collector manager
	collectorMgr := collector.NewManager(cfg, metricsChan, logger)

	// Create CSV exporter
	csvExporter, err := exporter.NewCSVExporter(cfg, metricsChan, logger)
	if err != nil {
		logger.Error("Failed to create CSV exporter", "error", err)
		return err
	}
	defer func() {
		if err := csvExporter.Close(); err != nil {
			logger.Error("Failed to close exporter", "error", err)
		}
	}()

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, initiating shutdown", "signal", sig)
		cancel()
	}()

	logger.Info("UnoStat is running", "output", cfg.OutputPath)

	// Use WaitGroup to track exporter goroutine
	var wg sync.WaitGroup

	// Start exporter goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := csvExporter.Start(ctx); err != nil {
			logger.Error("Exporter stopped with error", "error", err)
		}
	}()

	// Start collector manager (blocking until context is cancelled)
	if err := collectorMgr.Start(ctx); err != nil {
		logger.Error("Collector manager stopped with error", "error", err)
	}

	logger.Info("Shutting down...")

	// Wait for remaining metrics to be exported
	logger.Info("Waiting for remaining metrics to be exported...")
	time.Sleep(50 * time.Millisecond)

	// Close metrics channel to signal exporter to finish
	close(metricsChan)

	// Wait for exporter to finish draining the channel
	wg.Wait()

	logger.Info("Shutdown complete")

	return nil
}

// initLogger initializes the structured logger based on configuration.
func initLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler

	if cfg.LogFile != "" {
		// Log to file
		logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		// Use JSON handler for file logging
		handler = slog.NewJSONHandler(logFile, opts)
	} else {
		// Log to stdout
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// checkPlatformCapabilities logs platform-specific capability warnings.
func checkPlatformCapabilities(logger *slog.Logger) {
	switch runtime.GOOS {
	case osWindows:
		logger.Warn("Running on Windows: CPU iowait metric is not available")
	case osDarwin:
		logger.Info("Running on macOS: CPU iowait may have limited accuracy")
		logger.Info("Running on macOS: Disk metrics may require Full Disk Access or sudo")
	case osLinux:
		logger.Info("Running on Linux: All metrics available")
	default:
		logger.Warn("Running on unsupported platform, some metrics may not work", "os", runtime.GOOS)
	}
}
