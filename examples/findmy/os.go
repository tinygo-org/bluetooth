//go:build !tinygo

package main

import "os"

// PublicKey is the public key of the device. Must be base64 encoded.
var PublicKey = os.Args[1]
