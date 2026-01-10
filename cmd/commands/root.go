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
	"fmt"
	"log/slog"
	"os"

	"github.com/phuonguno98/unostat/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg *config.Config

	// Global persistent flags (shared by subcommands)
	logLevel string
	logFile  string
	timezone string
)

const (
	osWindows = "windows"
	osLinux   = "linux"
	osDarwin  = "darwin"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "unostat",
	Short: "UnoStat - Lightweight system performance monitoring tool",
	Long: `UnoStat is a lightweight, cross-platform system performance monitoring tool
written in Go. Specialized for performance testing to track real-time CPU, RAM,
Disk Utilization (busy time), Await, IOPS and Network Bandwidth with CSV export capability.

Use 'unostat collect' to begin monitoring.`,
	// No Version field here to direct user to version command
	// No RunE field, so it prints help by default
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		"Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "",
		"Log file path (empty = stdout)")
	rootCmd.PersistentFlags().StringVar(&timezone, "timezone", "Local",
		"Timezone for timestamps (e.g., 'Asia/Ho_Chi_Minh', 'Local')")
}

// InitLogger initializes and returns a slog.Logger based on the provided settings.
// It is shared by all commands to ensure consistent logging format.
func InitLogger(levelStr, fileStr string) *slog.Logger {
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
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
	if fileStr != "" {
		f, err := os.OpenFile(fileStr, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		handler = slog.NewJSONHandler(f, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
