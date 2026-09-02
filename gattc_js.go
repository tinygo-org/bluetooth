package bluetooth

import (
	"errors"
	"syscall/js"
)

var (
	_ GATTCService        = (*DeviceService)(nil)
	_ GATTCCharacteristic = (*DeviceCharacteristic)(nil)
)

// uuidWrapper is a type alias for UUID so we ensure no conflicts with
// struct method of the same name.
type uuidWrapper = UUID

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will return a complete list of
// services.
//
// The browser only returns a service that is in Adapter.RequestedServices.
func (d Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	if d.server.IsUndefined() {
		return nil, errors.New("bluetooth: not connected")
	}

	if len(uuids) == 0 {
		// Get all primary services.
		result, err := await(d.server.Call("getPrimaryServices"))
		if err != nil {
			return nil, err
		}

		length := result.Length()
		services := make([]DeviceService, length)
		for i := 0; i < length; i++ {
			jsSvc := result.Index(i)
			svcUUID, err := ParseUUID(jsSvc.Get("uuid").String())
			if err != nil {
				return nil, err
			}
			services[i] = DeviceService{
				deviceService: &deviceService{
					uuidWrapper: svcUUID,
					device:      d,
					service:     jsSvc,
				},
			}
		}
		return services, nil
	}

	// Get services by specific UUIDs, preserving order.
	services := make([]DeviceService, len(uuids))
	for i, uuid := range uuids {
		result, err := await(d.server.Call("getPrimaryService", uuid.String()))
		if err != nil {
			return nil, err
		}
		services[i] = DeviceService{
			deviceService: &deviceService{
				uuidWrapper: uuid,
				device:      d,
				service:     result,
			},
		}
	}
	return services, nil
}

// DeviceService is a BLE service on a connected peripheral device.
// It wraps a BluetoothRemoteGATTService JS object.
type DeviceService struct {
	*deviceService
}

type deviceService struct {
	uuidWrapper

	device  Device
	service js.Value // BluetoothRemoteGATTService
}

// UUID returns the UUID for this DeviceService.
func (s DeviceService) UUID() UUID {
	return s.uuidWrapper
}

// DiscoverCharacteristics discovers characteristics in this service. Pass a
// list of characteristic UUIDs you are interested in to this function. Either a
// list of all requested characteristics is returned, or if some could not be
// discovered an error is returned. If there is no error, the characteristics
// slice has the same length as the UUID slice with characteristics in the same
// order in the slice as in the requested UUID list.
//
// Passing a nil slice of UUIDs will return a complete list of
// characteristics.
func (s DeviceService) DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error) {
	if len(uuids) == 0 {
		result, err := await(s.service.Call("getCharacteristics"))
		if err != nil {
			return nil, err
		}

		length := result.Length()
		chars := make([]DeviceCharacteristic, length)
		for i := 0; i < length; i++ {
			jsChar := result.Index(i)
			cuuid, err := ParseUUID(jsChar.Get("uuid").String())
			if err != nil {
				return nil, err
			}
			chars[i] = DeviceCharacteristic{
				deviceCharacteristic: &deviceCharacteristic{
					uuidWrapper:    cuuid,
					service:        s,
					characteristic: jsChar,
				},
			}
		}
		return chars, nil
	}

	chars := make([]DeviceCharacteristic, len(uuids))
	for i, uuid := range uuids {
		result, err := await(s.service.Call("getCharacteristic", uuid.String()))
		if err != nil {
			return nil, err
		}
		chars[i] = DeviceCharacteristic{
			deviceCharacteristic: &deviceCharacteristic{
				uuidWrapper:    uuid,
				service:        s,
				characteristic: result,
			},
		}
	}
	return chars, nil
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device. It wraps a BluetoothRemoteGATTCharacteristic JS object.
type DeviceCharacteristic struct {
	*deviceCharacteristic
}

type deviceCharacteristic struct {
	uuidWrapper

	service        DeviceService
	characteristic js.Value // BluetoothRemoteGATTCharacteristic
	listener       js.Func
	listening      bool
}

// UUID returns the UUID for this DeviceCharacteristic.
func (c DeviceCharacteristic) UUID() UUID {
	return c.uuidWrapper
}

// Read reads the current characteristic value.
func (c DeviceCharacteristic) Read(data []byte) (int, error) {
	result, err := await(c.characteristic.Call("readValue"))
	if err != nil {
		return 0, err
	}

	buf := uint8Array(result)
	n := buf.Get("length").Int()
	if n > len(data) {
		n = len(data)
	}
	js.CopyBytesToGo(data[:n], buf)
	return n, nil
}

// Write replaces the characteristic value with a new value. The
// call will return after all data has been written.
func (c DeviceCharacteristic) Write(p []byte) (int, error) {
	jsArray := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(jsArray, p)
	_, err := await(c.characteristic.Call("writeValueWithResponse", jsArray.Get("buffer")))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteWithoutResponse replaces the characteristic value with a new value. The
// call will return before all data has been written.
func (c DeviceCharacteristic) WriteWithoutResponse(p []byte) (int, error) {
	jsArray := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(jsArray, p)
	_, err := await(c.characteristic.Call("writeValueWithoutResponse", jsArray.Get("buffer")))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// EnableNotifications enables notifications in the Client Characteristic
// Configuration Descriptor (CCCD). This means that most peripherals will send a
// notification with a new value every time the value of the characteristic
// changes.
//
// Users may call EnableNotifications with a nil callback to disable notifications.
func (c DeviceCharacteristic) EnableNotifications(callback func(buf []byte)) error {
	if callback == nil {
		// Remove the listener first so no event finds a nil callback.
		c.stopListening()
		_, err := await(c.characteristic.Call("stopNotifications"))
		return err
	}

	// Replace any listener from a previous call.
	c.stopListening()

	c.listener = js.FuncOf(func(this js.Value, args []js.Value) any {
		buf := uint8Array(args[0].Get("target").Get("value"))
		data := make([]byte, buf.Get("length").Int())
		js.CopyBytesToGo(data, buf)

		// A blocked JS callback stops the event loop, and every method in
		// this package waits for a promise. See syscall/js.FuncOf.
		go callback(data)
		return nil
	})
	c.characteristic.Call("addEventListener", "characteristicvaluechanged", c.listener)
	c.listening = true

	if _, err := await(c.characteristic.Call("startNotifications")); err != nil {
		c.stopListening()
		return err
	}
	return nil
}

// stopListening removes the value change listener and releases the JS function.
func (c *deviceCharacteristic) stopListening() {
	if !c.listening {
		return
	}
	c.characteristic.Call("removeEventListener", "characteristicvaluechanged", c.listener)
	c.listener.Release()
	c.listener = js.Func{}
	c.listening = false
}

// GetMTU returns the MTU for the characteristic.
//
// WebBluetooth has no method for the negotiated MTU, so this returns 512, the
// maximum length of an attribute value. The browser does the fragmentation.
//
// The writeValue method of the Web Bluetooth specification rejects a longer
// value: https://webbluetoothcg.github.io/web-bluetooth/
// See also Bluetooth Core Specification, Vol 3, Part F, section 3.2.9.
func (c DeviceCharacteristic) GetMTU() (uint16, error) {
	return 512, nil
}

// jsError converts the reason of a rejected Promise into an error.
func jsError(reason js.Value) error {
	if reason.IsUndefined() || reason.IsNull() {
		return errors.New("bluetooth: promise rejected without a reason")
	}
	return errors.New(reason.Call("toString").String())
}

// uint8Array wraps a DataView in a Uint8Array over the same bytes.
// A DataView can be a window into a larger ArrayBuffer.
func uint8Array(view js.Value) js.Value {
	return js.Global().Get("Uint8Array").New(
		view.Get("buffer"), view.Get("byteOffset"), view.Get("byteLength"))
}

// await blocks on a JavaScript Promise and returns (result, error).
func await(promise js.Value) (js.Value, error) {
	ch := make(chan js.Value, 1)
	errCh := make(chan error, 1)

	thenFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	})

	catchFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		var reason js.Value
		if len(args) > 0 {
			reason = args[0]
		}
		errCh <- jsError(reason)
		return nil
	})

	promise.Call("then", thenFunc).Call("catch", catchFunc)

	select {
	case val := <-ch:
		thenFunc.Release()
		catchFunc.Release()
		return val, nil
	case err := <-errCh:
		thenFunc.Release()
		catchFunc.Release()
		return js.Undefined(), err
	}
}
