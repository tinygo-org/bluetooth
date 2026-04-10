// This example demonstrates using the Web Bluetooth API via TinyGo WASM.
// It opens the browser's Bluetooth device picker, connects to any BLE
// peripheral, and reads the Device Information service (manufacturer name,
// model number, firmware revision).
//
// To build:
//
//	tinygo build -o ./examples/webbluetooth/html/wasm.wasm -target wasm ./examples/webbluetooth/
//
// Then serve the html directory over HTTPS (or localhost). A minimal server is
// included:
//
//	go run ./examples/webbluetooth/server/
//
// Open http://localhost:8080 in Chrome/Edge, click the "Connect" button and
// pick a Bluetooth device from the browser dialog.
package main

import (
	"syscall/js"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var (
	deviceInfoServiceUUID = bluetooth.ServiceUUIDDeviceInformation

	manufacturerNameUUID  = bluetooth.CharacteristicUUIDManufacturerNameString
	modelNumberUUID       = bluetooth.CharacteristicUUIDModelNumberString
	firmwareRevisionUUID  = bluetooth.CharacteristicUUIDFirmwareRevisionString
)

func main() {
	// Export a function that the HTML page calls when the user clicks Connect.
	js.Global().Set("btConnect", js.FuncOf(func(this js.Value, args []js.Value) any {
		go run()
		return nil
	}))

	logMsg("WebBluetooth example loaded. Click 'Connect' to start.")

	// Keep the Go program alive.
	select {}
}

func run() {
	logMsg("Enabling BLE adapter...")
	must("enable BLE stack", adapter.Enable())

	// WebBluetooth requires listing the services you want to access upfront.
	adapter.RequestedServices = []bluetooth.UUID{
		deviceInfoServiceUUID,
	}

	logMsg("Opening device picker...")

	var result bluetooth.ScanResult
	err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
		result = r
		a.StopScan()
	})
	must("scan", err)

	logMsg("Selected device: " + result.LocalName() + " (" + result.Address.String() + ")")

	// Connection can fail if the device is asleep or out of range. Retry a
	// few times before giving up.
	var device bluetooth.Device
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		logMsg("Connecting (attempt " + itoa(attempt) + "/" + itoa(maxRetries) + ")...")
		device, err = adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err == nil {
			break
		}
		logMsg("Connection failed: " + err.Error())
		if attempt == maxRetries {
			logMsg("Could not connect after " + itoa(maxRetries) + " attempts. Make sure the device is awake and in range, then click Connect again.")
			return
		}
	}
	logMsg("Connected!")

	logMsg("Discovering Device Information service...")
	srvcs, err := device.DiscoverServices([]bluetooth.UUID{deviceInfoServiceUUID})
	must("discover services", err)

	srvc := srvcs[0]
	logMsg("Found service: " + srvc.UUID().String())

	// Read each Device Information characteristic. Not all devices expose
	// every characteristic, so we read them individually and tolerate errors.
	buf := make([]byte, 128)

	for _, item := range []struct {
		name string
		uuid bluetooth.UUID
	}{
		{"Manufacturer Name", manufacturerNameUUID},
		{"Model Number", modelNumberUUID},
		{"Firmware Revision", firmwareRevisionUUID},
	} {
		chars, err := srvc.DiscoverCharacteristics([]bluetooth.UUID{item.uuid})
		if err != nil {
			logMsg("  " + item.name + ": not available")
			continue
		}
		n, err := chars[0].Read(buf)
		if err != nil {
			logMsg("  " + item.name + ": read error: " + err.Error())
			continue
		}
		logMsg("  " + item.name + ": " + string(buf[:n]))
	}

	logMsg("Disconnecting...")
	device.Disconnect()
	logMsg("Done.")
}

// logMsg writes a message to the browser console and appends it to the #log element.
func logMsg(msg string) {
	js.Global().Get("console").Call("log", msg)
	doc := js.Global().Get("document")
	logEl := doc.Call("getElementById", "log")
	if !logEl.IsNull() {
		p := doc.Call("createElement", "p")
		p.Set("textContent", msg)
		logEl.Call("appendChild", p)
	}
}

func must(action string, err error) {
	if err != nil {
		logMsg("FATAL: failed to " + action + ": " + err.Error())
		panic("failed to " + action + ": " + err.Error())
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
