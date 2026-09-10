// A static file server for the WebBluetooth example.
//
// See the README of the example for the build steps.
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	dir := flag.String("dir", "examples/webbluetooth/html", "directory to serve")
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	log.Printf("serving %s on http://localhost%s", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir))))
}
