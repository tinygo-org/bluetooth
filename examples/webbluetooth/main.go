// This example gives the WebBluetooth backend to the page as a small
// JavaScript API. The page in html/index.html does all of the display work.
//
// See the README for the build steps.
package main

import (
	"errors"
	"syscall/js"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var (
	device bluetooth.Device

	// discovery is slow, so keep each service and characteristic. Subscribe
	// and unsubscribe must also use the same characteristic value.
	services        = map[string]bluetooth.DeviceService{}
	characteristics = map[string]bluetooth.DeviceCharacteristic{}
)

func main() {
	ble := js.Global().Get("Object").New()
	ble.Set("enable", js.FuncOf(enable))
	ble.Set("requestDevice", js.FuncOf(requestDevice))
	ble.Set("connect", js.FuncOf(connect))
	ble.Set("disconnect", js.FuncOf(disconnect))
	ble.Set("read", js.FuncOf(read))
	ble.Set("readString", js.FuncOf(readString))
	ble.Set("subscribe", js.FuncOf(subscribe))
	ble.Set("unsubscribe", js.FuncOf(unsubscribe))
	ble.Set("onConnectionChange", js.FuncOf(onConnectionChange))
	js.Global().Set("ble", ble)

	// Tell the page that the API is ready.
	if hook := js.Global().Get("onBleReady"); hook.Type() == js.TypeFunction {
		hook.Invoke()
	}

	// Keep the Go program alive.
	select {}
}

// enable prepares the adapter. It returns a Promise.
func enable(this js.Value, args []js.Value) any {
	return promise(func() (any, error) {
		return nil, adapter.Enable()
	})
}

// requestDevice opens the device picker and returns a Promise for an object
// with the id and the name of the device that the user selects.
func requestDevice(this js.Value, args []js.Value) any {
	var wanted []string
	if list := arg(args, 0); list.Type() == js.TypeObject {
		for i := 0; i < list.Length(); i++ {
			wanted = append(wanted, list.Index(i).String())
		}
	}

	return promise(func() (any, error) {
		uuids := make([]bluetooth.UUID, len(wanted))
		for i, s := range wanted {
			uuid, err := bluetooth.ParseUUID(s)
			if err != nil {
				return nil, err
			}
			uuids[i] = uuid
		}

		// The browser refuses access to a service that is not in this list.
		adapter.RequestedServices = uuids

		var result bluetooth.ScanResult
		err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
			result = r
			a.StopScan()
		})
		if err != nil {
			return nil, err
		}

		found := js.Global().Get("Object").New()
		found.Set("id", result.Address.String())
		found.Set("name", result.LocalName())
		return found, nil
	})
}

// connect connects to the device with the given id. It returns a Promise.
func connect(this js.Value, args []js.Value) any {
	id := arg(args, 0).String()

	return promise(func() (any, error) {
		var address bluetooth.Address
		address.Set(id)

		d, err := adapter.Connect(address, bluetooth.ConnectionParams{})
		if err != nil {
			return nil, err
		}
		device = d
		clearCache()
		return nil, nil
	})
}

// disconnect closes the connection. It returns a Promise.
func disconnect(this js.Value, args []js.Value) any {
	return promise(func() (any, error) {
		err := device.Disconnect()
		clearCache()
		return nil, err
	})
}

// read returns a Promise for the value of a characteristic as a Uint8Array.
func read(this js.Value, args []js.Value) any {
	serviceUUID, charUUID := arg(args, 0).String(), arg(args, 1).String()

	return promise(func() (any, error) {
		buf, err := readValue(serviceUUID, charUUID)
		if err != nil {
			return nil, err
		}
		value := js.Global().Get("Uint8Array").New(len(buf))
		js.CopyBytesToJS(value, buf)
		return value, nil
	})
}

// readString returns a Promise for the value of a characteristic as a string.
func readString(this js.Value, args []js.Value) any {
	serviceUUID, charUUID := arg(args, 0).String(), arg(args, 1).String()

	return promise(func() (any, error) {
		buf, err := readValue(serviceUUID, charUUID)
		if err != nil {
			return nil, err
		}
		return string(buf), nil
	})
}

// subscribe starts notifications and calls the callback with a Uint8Array for
// each new value. It returns a Promise.
func subscribe(this js.Value, args []js.Value) any {
	serviceUUID, charUUID := arg(args, 0).String(), arg(args, 1).String()
	callback := arg(args, 2)

	return promise(func() (any, error) {
		if callback.Type() != js.TypeFunction {
			return nil, errors.New("subscribe needs a callback function")
		}
		char, err := characteristic(serviceUUID, charUUID)
		if err != nil {
			return nil, err
		}
		return nil, char.EnableNotifications(func(buf []byte) {
			value := js.Global().Get("Uint8Array").New(len(buf))
			js.CopyBytesToJS(value, buf)
			callback.Invoke(value)
		})
	})
}

// unsubscribe stops notifications. It returns a Promise.
func unsubscribe(this js.Value, args []js.Value) any {
	serviceUUID, charUUID := arg(args, 0).String(), arg(args, 1).String()

	return promise(func() (any, error) {
		char, err := characteristic(serviceUUID, charUUID)
		if err != nil {
			return nil, err
		}
		return nil, char.EnableNotifications(nil)
	})
}

// onConnectionChange calls the callback with a boolean on each connection and
// disconnection. Call it before connect.
func onConnectionChange(this js.Value, args []js.Value) any {
	callback := arg(args, 0)
	if callback.Type() != js.TypeFunction {
		return nil
	}

	adapter.SetConnectHandler(func(d bluetooth.Device, connected bool) {
		callback.Invoke(connected)
	})
	return nil
}

// readValue reads a characteristic into a buffer of the maximum ATT MTU.
func readValue(serviceUUID, charUUID string) ([]byte, error) {
	char, err := characteristic(serviceUUID, charUUID)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 512)
	n, err := char.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// characteristic finds a characteristic and keeps it for the next call.
func characteristic(serviceUUID, charUUID string) (bluetooth.DeviceCharacteristic, error) {
	key := serviceUUID + "/" + charUUID
	if char, ok := characteristics[key]; ok {
		return char, nil
	}

	svc, err := service(serviceUUID)
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, err
	}
	uuid, err := bluetooth.ParseUUID(charUUID)
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, err
	}
	found, err := svc.DiscoverCharacteristics([]bluetooth.UUID{uuid})
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, err
	}

	characteristics[key] = found[0]
	return found[0], nil
}

// service finds a service and keeps it for the next call.
func service(serviceUUID string) (bluetooth.DeviceService, error) {
	if svc, ok := services[serviceUUID]; ok {
		return svc, nil
	}

	uuid, err := bluetooth.ParseUUID(serviceUUID)
	if err != nil {
		return bluetooth.DeviceService{}, err
	}
	found, err := device.DiscoverServices([]bluetooth.UUID{uuid})
	if err != nil {
		return bluetooth.DeviceService{}, err
	}

	services[serviceUUID] = found[0]
	return found[0], nil
}

func clearCache() {
	services = map[string]bluetooth.DeviceService{}
	characteristics = map[string]bluetooth.DeviceCharacteristic{}
}

// arg returns the argument at index i, or undefined.
func arg(args []js.Value, i int) js.Value {
	if i < len(args) {
		return args[i]
	}
	return js.Undefined()
}

// promise runs fn on a new goroutine and settles the Promise that it returns.
//
// A function that JavaScript calls must not block, because it holds the event
// loop until it returns. See syscall/js.FuncOf.
func promise(fn func() (any, error)) js.Value {
	executor := js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			value, err := fn()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolve.Invoke(value)
		}()
		return nil
	})

	// The Promise constructor calls the executor before it returns.
	defer executor.Release()
	return js.Global().Get("Promise").New(executor)
}
