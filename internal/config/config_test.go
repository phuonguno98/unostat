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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "Single value",
			input:    "sda",
			expected: []string{"sda"},
		},
		{
			name:     "Multiple values",
			input:    "sda,sdb",
			expected: []string{"sda", "sdb"},
		},
		{
			name:     "Whitespace handling",
			input:    " sda , sdb ",
			expected: []string{"sda", "sdb"},
		},
		{
			name:     "Empty parts",
			input:    "sda,,sdb",
			expected: []string{"sda", "sdb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCommaSeparated(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("ParseCommaSeparated() length = %v, want %v", len(got), len(tt.expected))
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("ParseCommaSeparated()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	validOutputPath := filepath.Join(tempDir, "test.csv")

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: Config{
				SamplingInterval: 5 * time.Second,
				OutputPath:       validOutputPath,
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
			},
			wantErr: false,
		},
		{
			name: "Invalid Sampling Interval (Too small)",
			config: Config{
				SamplingInterval: 500 * time.Millisecond,
				OutputPath:       validOutputPath,
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "Invalid Sampling Interval (Too large)",
			config: Config{
				SamplingInterval: 2 * time.Hour,
				OutputPath:       validOutputPath,
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "Empty Output Path",
			config: Config{
				SamplingInterval: 5 * time.Second,
				OutputPath:       "",
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "Invalid Buffer Size",
			config: Config{
				SamplingInterval: 5 * time.Second,
				OutputPath:       validOutputPath,
				BufferSize:       0,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "Invalid Log Level",
			config: Config{
				SamplingInterval: 5 * time.Second,
				OutputPath:       validOutputPath,
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "invalid",
			},
			wantErr: true,
		},
		{
			name: "Valid Timezone",
			config: Config{
				SamplingInterval: 5 * time.Second,
				OutputPath:       validOutputPath,
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
				Timezone:         "UTC",
			},
			wantErr: false,
		},
		{
			name: "Invalid Timezone",
			config: Config{
				SamplingInterval: 5 * time.Second,
				OutputPath:       validOutputPath,
				BufferSize:       100,
				FlushInterval:    5 * time.Second,
				LogLevel:         "info",
				Timezone:         "Invalid/Timezone",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDefaultOutputPath(t *testing.T) {
	path := GetDefaultOutputPath()
	if path == "" {
		t.Error("GetDefaultOutputPath() returned empty string")
	}
	if !strings.HasSuffix(path, ".csv") {
		t.Errorf("GetDefaultOutputPath() = %v, expected .csv suffix", path)
	}
}

func TestLoadFromArgs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unostat_args_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	validOutputPath := filepath.Join(tempDir, "output.csv")

	tests := []struct {
		name        string
		args        []string
		expected    *Config
		expectError bool
	}{
		{
			name: "Defaults",
			args: []string{},
			expected: &Config{
				SamplingInterval: DefaultSamplingInterval,
				BufferSize:       DefaultBufferSize,
				LogLevel:         DefaultLogLevel,
			},
			expectError: false,
		},
		{
			name: "Custom Values",
			args: []string{
				"-interval", "10s",
				"-output", validOutputPath,
				"-buffer-size", "50",
				"-log-level", "debug",
				"-include-disks", "sda,sdb",
			},
			expected: &Config{
				SamplingInterval: 10 * time.Second,
				OutputPath:       validOutputPath,
				BufferSize:       50,
				LogLevel:         "debug",
				IncludeDisks:     []string{"sda", "sdb"},
			},
			expectError: false,
		},
		{
			name:        "Unknown Flag",
			args:        []string{"-unknown-flag"},
			expectError: true,
		},
		{
			name: "Invalid Config (Validation Failure)",
			args: []string{
				"-interval", "100ms", // To small, validation should fail
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadFromArgs(tt.args)
			if tt.expectError {
				if err == nil {
					t.Error("LoadFromArgs() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("LoadFromArgs() unexpected error: %v", err)
				return
			}

			// Validate key fields
			if cfg.SamplingInterval != tt.expected.SamplingInterval {
				t.Errorf("SamplingInterval = %v, want %v", cfg.SamplingInterval, tt.expected.SamplingInterval)
			}
			if tt.expected.OutputPath != "" && cfg.OutputPath != tt.expected.OutputPath {
				t.Errorf("OutputPath = %v, want %v", cfg.OutputPath, tt.expected.OutputPath)
			}
			if tt.expected.BufferSize != 0 && cfg.BufferSize != tt.expected.BufferSize {
				t.Errorf("BufferSize = %v, want %v", cfg.BufferSize, tt.expected.BufferSize)
			}
			if len(tt.expected.IncludeDisks) > 0 {
				if len(cfg.IncludeDisks) != len(tt.expected.IncludeDisks) {
					t.Errorf("IncludeDisks count = %v, want %v", len(cfg.IncludeDisks), len(tt.expected.IncludeDisks))
				}
			}

			// Additional check for defaults if not specified
			if cfg.OutputPath == "" {
				t.Error("OutputPath is empty")
			}
		})
	}
}
