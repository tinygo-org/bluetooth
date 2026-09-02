# WebBluetooth example

This example connects to a BLE device from a web page. It opens the device picker of the browser, connects to the device that you select, and reads the manufacturer name, the model number and the firmware revision from the Device Information service.

## Requirements

* [TinyGo](https://tinygo.org/getting-started/install/), or the standard Go toolchain.
* A browser with WebBluetooth support, such as Chrome or Edge. See the [browser compatibility table](https://developer.mozilla.org/en-US/docs/Web/API/Web_Bluetooth_API#browser_compatibility).

WebBluetooth is only available in a secure context. A page on `localhost` counts as secure, so you do not need HTTPS during development.

## Build and run

The page needs two build products in the `html` directory: the module `wasm.wasm`, and the JavaScript support file `wasm_exec.js`. TinyGo and the standard Go toolchain each have their own support file. Always take both products from the same toolchain. See [Mixed toolchains](#mixed-toolchains) for the error that a mixed pair gives.

Run all commands from the root of the repository.

### Build with TinyGo

```shell
cp "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" ./examples/webbluetooth/html/
tinygo build -o ./examples/webbluetooth/html/wasm.wasm -target wasm ./examples/webbluetooth/
```

### Build with the standard Go toolchain

```shell
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" ./examples/webbluetooth/html/
GOOS=js GOARCH=wasm go build -o ./examples/webbluetooth/html/wasm.wasm ./examples/webbluetooth/
```

Go versions before 1.24 keep the support file in `misc/wasm/wasm_exec.js`.

### Windows

The TinyGo commands also work in PowerShell. The standard Go toolchain needs the two variables in the environment:

```powershell
$env:GOOS = "js"; $env:GOARCH = "wasm"
go build -o ./examples/webbluetooth/html/wasm.wasm ./examples/webbluetooth/
```

### Serve the page

```shell
go run ./examples/webbluetooth/server/
```

Open http://localhost:8080 and click **Connect**. The browser shows the device picker. Select a device to see the result in the page.

Both `wasm.wasm` and `wasm_exec.js` are build output, so git ignores them.

## Mixed toolchains

A module from TinyGo also imports `wasi_snapshot_preview1`, but the support file of the standard Go toolchain only gives `gojs`. The browser then shows this error:

```
Uncaught (in promise) TypeError: WebAssembly.instantiate(): Import #1 "wasi_snapshot_preview1": module is not an object or function
```

To repair it, copy the support file of the toolchain that built the module, and reload the page. An old `wasm_exec.js` stays in the `html` directory because git ignores it, so copy the file again after you change toolchain.

## Limitations of WebBluetooth

* The browser gives an opaque device ID instead of a MAC address. The `Address` type holds this ID.
* You must call `Scan` before `Connect`. The browser object for the device does not survive a page reload.
* You must list every service that you want to use in `Adapter.RequestedServices` before you call `Scan`. The browser refuses access to a service that is not in the list.
* The browser does not report the RSSI of the selected device.
* The browser does not give the negotiated MTU. `GetMTU` returns 512.
* WebBluetooth has no peripheral role, so advertisement and local services are not available.
