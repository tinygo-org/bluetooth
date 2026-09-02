# WebBluetooth example

This example connects to a BLE device from a web page. It opens the device
picker of the browser, connects to the device that you select, and reads the
manufacturer name, the model number and the firmware revision from the Device
Information service.

## Requirements

* [TinyGo](https://tinygo.org/getting-started/install/), or the standard Go
  toolchain.
* A browser with WebBluetooth support, such as Chrome or Edge. See the
  [browser compatibility table](https://developer.mozilla.org/en-US/docs/Web/API/Web_Bluetooth_API#browser_compatibility).

WebBluetooth is only available in a secure context. A page on `localhost`
counts as secure, so you do not need HTTPS during development.

## Build and run

Run all commands from the root of the repository.

1. Copy the JavaScript support file into the `html` directory. TinyGo and the
   standard Go toolchain each have their own file, and the two are not
   interchangeable.

   With TinyGo:

   ```shell
   cp "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" ./examples/webbluetooth/html/
   ```

   With the standard Go toolchain:

   ```shell
   cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" ./examples/webbluetooth/html/
   ```

   Go versions before 1.24 keep the file in `misc/wasm/wasm_exec.js`.

2. Build the WASM module.

   With TinyGo:

   ```shell
   tinygo build -o ./examples/webbluetooth/html/wasm.wasm -target wasm ./examples/webbluetooth/
   ```

   With the standard Go toolchain:

   ```shell
   GOOS=js GOARCH=wasm go build -o ./examples/webbluetooth/html/wasm.wasm ./examples/webbluetooth/
   ```

3. Start the file server.

   ```shell
   go run ./examples/webbluetooth/server/
   ```

4. Open http://localhost:8080 and click **Connect**. The browser shows the
   device picker. Select a device to see the result in the page.

Both `wasm.wasm` and `wasm_exec.js` are build output, so git ignores them.

### Windows

PowerShell uses different commands for step 1 and step 2:

```powershell
Copy-Item "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" ./examples/webbluetooth/html/
tinygo build -o ./examples/webbluetooth/html/wasm.wasm -target wasm ./examples/webbluetooth/
```

## Limitations of WebBluetooth

* The browser gives an opaque device ID instead of a MAC address. The
  `Address` type holds this ID.
* You must call `Scan` before `Connect`. The browser object for the device
  does not survive a page reload.
* You must list every service that you want to use in
  `Adapter.RequestedServices` before you call `Scan`. The browser refuses
  access to a service that is not in the list.
* The browser does not report the RSSI of the selected device.
* The browser does not give the negotiated MTU. `GetMTU` returns 512.
* WebBluetooth has no peripheral role, so advertisement and local services are
  not available.
