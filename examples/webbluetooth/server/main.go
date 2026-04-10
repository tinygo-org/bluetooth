// A minimal HTTP server for the WebBluetooth example.
//
// Usage:
//
//	go run ./examples/webbluetooth/server/
//
// Then open http://localhost:8080 in Chrome or Edge.
// WebBluetooth works on localhost without HTTPS.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	dir := findHTMLDir()

	ensureWASMExecJS(dir)
	ensureWASM(dir)

	addr := ":8080"
	fmt.Printf("Serving %s on http://localhost%s\n", dir, addr)
	log.Fatal(http.ListenAndServe(addr, http.FileServer(http.Dir(dir))))
}

// findHTMLDir locates the html/ directory relative to this file or cwd.
func findHTMLDir() string {
	// Try relative to the working directory first.
	candidates := []string{
		"examples/webbluetooth/html",
		"html",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	fmt.Fprintln(os.Stderr, "Could not find html/ directory. Run from the repo root or the example directory.")
	os.Exit(1)
	return ""
}

// ensureWASMExecJS copies the TinyGo wasm_exec.js support file into the html directory.
func ensureWASMExecJS(dir string) {
	dst := filepath.Join(dir, "wasm_exec.js")
	if _, err := os.Stat(dst); err == nil {
		return // already exists
	}

	// Try TinyGo's wasm_exec.js first (targets/wasm_exec.js).
	if tinygoRoot := os.Getenv("TINYGOROOT"); tinygoRoot != "" {
		src := filepath.Join(tinygoRoot, "targets", "wasm_exec.js")
		if copyFile(src, dst) == nil {
			fmt.Println("Copied wasm_exec.js from TINYGOROOT")
			return
		}
	}

	// Try to find tinygo in PATH.
	if tinygo, err := exec.LookPath("tinygo"); err == nil {
		// tinygo env TINYGOROOT
		out, err := exec.Command(tinygo, "env", "TINYGOROOT").Output()
		if err == nil {
			root := strings.TrimSpace(string(out))
			src := filepath.Join(root, "targets", "wasm_exec.js")
			if copyFile(src, dst) == nil {
				fmt.Println("Copied wasm_exec.js from tinygo in PATH")
				return
			}
		}
	}

	// Last resort: standard Go's wasm_exec.js. Note: this will NOT work
	// with TinyGo-built WASM because it lacks WASI shims.
	goroot := runtime.GOROOT()
	src := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	if copyFile(src, dst) == nil {
		fmt.Println("WARNING: copied wasm_exec.js from GOROOT — this will NOT work with TinyGo WASM.")
		fmt.Println("Install TinyGo or set TINYGOROOT to get the correct wasm_exec.js.")
		return
	}

	fmt.Fprintln(os.Stderr, "Warning: could not find wasm_exec.js. Copy it manually into", dir)
	fmt.Fprintln(os.Stderr, "  cp $(tinygo env TINYGOROOT)/targets/wasm_exec.js", dst)
}

// ensureWASM checks that wasm.wasm exists, and prints a hint if not.
func ensureWASM(dir string) {
	dst := filepath.Join(dir, "wasm.wasm")
	if _, err := os.Stat(dst); err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Warning: wasm.wasm not found in", dir)
	fmt.Fprintln(os.Stderr, "Build it with:")
	fmt.Fprintln(os.Stderr, "  tinygo build -o "+dst+" -target wasm ./examples/webbluetooth/")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
