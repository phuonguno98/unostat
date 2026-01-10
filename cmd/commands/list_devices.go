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
	"os"

	"github.com/phuonguno98/unostat/internal/devices"
	"github.com/spf13/cobra"
)

var listDevicesCmd = &cobra.Command{
	Use:   "list-devices",
	Short: "List available disk devices and network interfaces",
	Long: `List all available disk devices and network interfaces on the system.
This helps to configure include/exclude filters accurately.

Examples:
  # List all available devices
  unostat list-devices

  # Use the output to configure filters
  unostat --include-disks="C:" --exclude-networks="Loopback"`,
	RunE: runListDevices,
}

func init() {
	rootCmd.AddCommand(listDevicesCmd)
}

func runListDevices(cmd *cobra.Command, args []string) error {
	fmt.Println("\n========================================")
	fmt.Println("   UnoStat - Available Devices")
	fmt.Println("========================================")

	// List disk devices
	disks, err := devices.ListDisks()
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error listing disks: %v\n", err)
	case len(disks) == 0:
		fmt.Println("\nNo disk devices found.")
	default:
		fmt.Print(devices.FormatDisksTable(disks))
		fmt.Println("\nExample usage:")
		if len(disks) > 0 {
			fmt.Printf("  unostat --include-disks=\"%s\"\n", disks[0].Name)
		}
		if len(disks) > 1 {
			fmt.Printf("  unostat --exclude-disks=\"%s\"\n", disks[1].Name)
		}
	}

	// List network interfaces
	networks, err := devices.ListNetworkInterfaces()
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error listing network interfaces: %v\n", err)
	case len(networks) == 0:
		fmt.Println("\nNo network interfaces found.")
	default:
		fmt.Print(devices.FormatNetworksTable(networks))
		fmt.Println("\nExample usage:")
		if len(networks) > 0 {
			fmt.Printf("  unostat --include-networks=\"%s\"\n", networks[0].Name)
		}
		if len(networks) > 1 {
			fmt.Printf("  unostat --exclude-networks=\"%s\"\n", networks[1].Name)
		}
	}

	fmt.Println("\nNotes:")
	fmt.Println("  - Use comma to separate multiple devices: --exclude-disks=\"dev1,dev2\"")
	fmt.Println("  - Exclude filters take priority over include filters")
	fmt.Println("  - Empty include list means monitor all devices (except excluded)")
	fmt.Println()

	return nil
}
