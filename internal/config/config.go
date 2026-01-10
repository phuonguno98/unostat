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

package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents application configuration.
type Config struct {
	SamplingInterval time.Duration // Interval between metric collections
	OutputPath       string        // Path to CSV output file
	BufferSize       int           // Number of records to buffer before flush
	FlushInterval    time.Duration // Maximum time before forcing a flush

	// Filters
	IncludeDisks    []string // Disk devices to monitor (empty = all)
	ExcludeDisks    []string // Disk devices to exclude
	IncludeNetworks []string // Network interfaces to monitor (empty = all)
	ExcludeNetworks []string // Network interfaces to exclude

	// Logging
	LogLevel string // Log level: debug, info, warn, error
	LogFile  string // Log file path (empty = stdout)

	// Timezone
	Timezone string // Timezone location (e.g., "Asia/Ho_Chi_Minh", "Local")

	// Commands
	ListDevices bool // List available disks and network interfaces
}

// Default configuration values.
const (
	DefaultSamplingInterval  = 30 * time.Second
	DefaultBufferSize        = 100
	DefaultFlushInterval     = 5 * time.Second
	DefaultLogLevel          = "info"
	DefaultMaxOutputFileSize = 150 * 1024 * 1024 // 150MB
)

// GetDefaultOutputPath generates default output path: <hostname>_<timestamp>.csv
func GetDefaultOutputPath() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Clean hostname (remove invalid filename characters)
	hostname = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, hostname)

	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.csv", hostname, timestamp)

	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to current directory
		return filename
	}

	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, filename)
}

// LoadFromFlags loads configuration from command-line flags.
func LoadFromFlags() (*Config, error) {
	return LoadFromArgs(os.Args[1:])
}

// LoadFromArgs loads configuration from the provided arguments.
func LoadFromArgs(args []string) (*Config, error) {
	cfg := &Config{}
	fs := flag.NewFlagSet("unostat", flag.ContinueOnError)

	var (
		samplingInterval = fs.Duration("interval", DefaultSamplingInterval, "Sampling interval (e.g., 1s, 30s, 1m)")
		outputPath       = fs.String("output", "", "Output CSV file path (default: <hostname>_<timestamp>.csv)")
		bufferSize       = fs.Int("buffer-size", DefaultBufferSize, "Buffer size for CSV writer")
		flushInterval    = fs.Duration("flush-interval", DefaultFlushInterval, "Flush interval for CSV writer")

		logLevel = fs.String("log-level", DefaultLogLevel, "Log level (debug, info, warn, error)")
		logFile  = fs.String("log-file", "", "Log file path (empty = stdout)")

		includeDisks    = fs.String("include-disks", "", "Comma-separated list of disk devices to monitor (empty = all)")
		excludeDisks    = fs.String("exclude-disks", "", "Comma-separated list of disk devices to exclude")
		includeNetworks = fs.String("include-networks", "", "Comma-separated list of network interfaces to monitor (empty = all)")
		excludeNetworks = fs.String("exclude-networks", "", "Comma-separated list of network interfaces to exclude")

		listDevices = fs.Bool("list-devices", false, "List available disk and network devices, then exit")
	)

	// Parse arguments
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.SamplingInterval = *samplingInterval
	cfg.BufferSize = *bufferSize
	cfg.FlushInterval = *flushInterval
	cfg.LogLevel = *logLevel
	cfg.LogFile = *logFile
	cfg.ListDevices = *listDevices

	// Set output path (use default if not specified)
	if *outputPath == "" {
		cfg.OutputPath = GetDefaultOutputPath()
	} else {
		cfg.OutputPath = *outputPath
	}

	// Parse filter lists
	cfg.IncludeDisks = parseCommaSeparated(*includeDisks)
	cfg.ExcludeDisks = parseCommaSeparated(*excludeDisks)
	cfg.IncludeNetworks = parseCommaSeparated(*includeNetworks)
	cfg.ExcludeNetworks = parseCommaSeparated(*excludeNetworks)

	// Skip validation if just listing devices
	if cfg.ListDevices {
		return cfg, nil
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// parseCommaSeparated parses a comma-separated string into a slice of trimmed strings.
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// ParseCommaSeparated is the exported version of parseCommaSeparated.
func ParseCommaSeparated(s string) []string {
	return parseCommaSeparated(s)
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.SamplingInterval < 1*time.Second {
		return errors.New("sampling interval must be at least 1 second")
	}

	if c.SamplingInterval > 1*time.Hour {
		return errors.New("sampling interval must not exceed 1 hour")
	}

	if c.OutputPath == "" {
		return errors.New("output path cannot be empty")
	}

	if c.BufferSize < 1 {
		return errors.New("buffer size must be at least 1")
	}

	if c.FlushInterval < 1*time.Second {
		return errors.New("flush interval must be at least 1 second")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate Timezone
	if c.Timezone != "" {
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s (%w)", c.Timezone, err)
		}
	}

	// Check if output directory exists
	if err := c.ensureOutputDir(); err != nil {
		return fmt.Errorf("output directory check failed: %w", err)
	}

	return nil
}

// ensureOutputDir checks if the output directory exists and is writable.
func (c *Config) ensureOutputDir() error {
	dir := c.OutputPath

	// Get directory path
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' || dir[i] == '\\' {
			dir = dir[:i]
			break
		}
	}

	// If no directory separator found, use current directory
	if dir == c.OutputPath {
		dir = "."
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", dir)
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("output path parent is not a directory: %s", dir)
	}

	return nil
}

// String returns a human-readable representation of the configuration.
func (c *Config) String() string {
	return fmt.Sprintf("Config{Interval=%v, Output=%s, BufferSize=%d, FlushInterval=%v}, Timezone=%s",
		c.SamplingInterval, c.OutputPath, c.BufferSize, c.FlushInterval, c.Timezone)
}
