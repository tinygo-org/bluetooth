//go:build !baremetal

package main

import "os"

func connectAddress() string {
	if len(os.Args) < 2 {
		println("usage: heartrate-monitor [address]")
		os.Exit(1)
	}

	// look for device with specific name
	address := os.Args[1]

	return address
}

// done just prints a message and allows program to exit.
func done() {
	println("Done.")
}
