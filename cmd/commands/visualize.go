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
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/phuonguno98/unostat/internal/server"
	"github.com/spf13/cobra"
)

var (
	// Visualize command specific flags
	visPort        int
	visHost        string
	visUploadDir   string
	visOpenBrowser bool
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Start the visualization dashboard",
	Long: `Start the web-based dashboard for visualizing performance metrics.
Upload CSV files and explore your metrics with interactive charts.

Features:
  • Upload multiple CSV files
  • Interactive time-series charts for all metrics
  • Time range filtering
  • Export charts to PNG
  • Fully embedded in the binary

Examples:
  # Start server on default port 8080
  unostat visualize

  # Start on localhost only
  unostat visualize --host 127.0.0.1 --port 3000`,

	RunE: runVisualize,
}

func init() {
	rootCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().StringVar(&visHost, "host", "0.0.0.0", "HTTP server listen address")
	visualizeCmd.Flags().IntVarP(&visPort, "port", "p", 8080, "HTTP server port")
	visualizeCmd.Flags().StringVarP(&visUploadDir, "upload-dir", "d", "", "Directory to store uploaded CSV files (default: uploads)")
	visualizeCmd.Flags().BoolVar(&visOpenBrowser, "open-browser", false, "Open browser automatically after server starts")
}

// createServerInstance encapsulates server creation logic for testing.
func createServerInstance(uploadDir string, tz string, logger *slog.Logger) (*server.Server, error) {
	// Set default upload directory
	if uploadDir == "" {
		uploadDir = getDefaultUploadDir()
	}

	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve upload directory: %w", err)
	}

	return server.NewServer(absUploadDir, tz, logger)
}

func runVisualize(_ *cobra.Command, _ []string) error {
	// Initialize logger (reuse logic similar to start command but we can simple it here or respect globals)
	// We will respect global 'logLevel' and 'logFile' from root.go
	logger := InitLogger(logLevel, logFile)

	logger.Info("Starting UnoStat Dashboard",
		"host", visHost,
		"port", visPort,
	)

	// Create server instance (use global timezone from root command)
	server, err := createServerInstance(visUploadDir, timezone, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", visHost, visPort),
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, initiating shutdown", "signal", sig)
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server shutdown error", "error", err)
		}
	}()

	serverURL := fmt.Sprintf("http://localhost:%d", visPort)
	if visHost != "0.0.0.0" {
		serverURL = fmt.Sprintf("http://%s:%d", visHost, visPort)
	}

	fmt.Printf("\nUnoStat Dashboard is running!\n")
	fmt.Printf("URL: %s\n", serverURL)
	fmt.Printf("Uploads: %s\n\n", server.UploadDir())

	if visOpenBrowser {
		go openBrowserURL(serverURL)
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	<-ctx.Done()
	logger.Info("Server stopped")
	return nil
}

func getDefaultUploadDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "uploads"
	}
	return filepath.Join(filepath.Dir(exePath), "uploads")
}

func openBrowserURL(url string) {
	time.Sleep(500 * time.Millisecond)
	var cmd *exec.Cmd
	switch {
	case fileExists("C:\\Windows\\System32\\rundll32.exe"):
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case fileExists("/usr/bin/xdg-open"):
		cmd = exec.Command("xdg-open", url)
	case fileExists("/usr/bin/open"):
		cmd = exec.Command("open", url)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		// Ignore errors, browser opening is optional.
		// Just print to stderr for debugging if needed.
		fmt.Fprintf(os.Stderr, "Failed to open browser: %v\n", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
