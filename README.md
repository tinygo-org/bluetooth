# Go Bluetooth

[![PkgGoDev](https://pkg.go.dev/badge/pkg.go.dev/github.com/tinygo-org/bluetooth)](https://pkg.go.dev/pkg.go.dev/github.com/tinygo-org/bluetooth)
[![CircleCI](https://circleci.com/gh/tinygo-org/bluetooth/tree/dev.svg?style=svg)](https://circleci.com/gh/tinygo-org/bluetooth/tree/dev)

This package provides a cross-platform Bluetooth Low Energy module for Go that can be used on operating systems such as Linux, macOS, and Windows. 

It can also be used running "bare metal" on microcontrollers such as those produced by Nordic Semiconductor.

This example scans for peripheral devices and then displays information about them as they are discovered:

```go
package main

import (
	"github.com/tinygo-org/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Start scanning.
	println("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		println("found device:", device.Address.String(), device.RSSI, device.LocalName())
	})
	must("start scan", err)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
```

## Current support

|                                  | Linux              | macOS              | Windows            | Nordic Semi        |
| -------------------------------- | ------------------ | ------------------ | ------------------ | ------------------ |
| API used                         | BlueZ              | CoreBluetooth      | WinRT              | SoftDevice         |
| Scanning                         | :heavy_check_mark: | :heavy_check_mark: | :heavy_check_mark: | :heavy_check_mark: |
| Connect to peripheral            | :heavy_check_mark: | :heavy_check_mark: | :x:                | :heavy_check_mark: |
| Write peripheral characteristics | :heavy_check_mark: | :heavy_check_mark: | :x:                | :heavy_check_mark: |
| Receive notifications            | :heavy_check_mark: | :heavy_check_mark: | :x:                | :heavy_check_mark: |
| Advertisement                    | :heavy_check_mark: | :x:                | :x:                | :heavy_check_mark: |
| Local services                   | :heavy_check_mark: | :x:                | :x:                | :heavy_check_mark: |
| Local characteristics            | :heavy_check_mark: | :x:                | :x:                | :heavy_check_mark: |
| Send notifications               | :heavy_check_mark: | :x:                | :x:                | :heavy_check_mark: |

## Linux

The current support for Linux uses BlueZ via the D-Bus interface. It should work with most distros that support BlueZ such as Ubuntu, Debian, Fedora, and Arch Linux, among others. Linux can be used both as a BLE central as well as BLE peripheral.

## macOS

The current macOS support uses the CoreBluetooth libraries provided by macOS. As a result, it should work with most versions of macOS, although it will require compiling using whatever specific version of XCode is required by your version of the operating system. The macOS support only can only act as a BLE central at this time, with some additional development support needed for full functionality.

## Windows

The Windows support is still experimental, and needs additional development to be useful.

## Nordic Semiconductor

As you can see above, there is bare metal support for several chips from Nordic Semiconductors.

These chips are supported through [TinyGo](https://tinygo.org/).

This support also requires firmware provided by Nordic Semi known as the "SoftDevice". The SoftDevice is a binary blob that implements the BLE stack. There are other (open source) BLE stacks, but the SoftDevices are pretty solid and have all the qualifications you might need. Other BLE stacks might be added in the future.

At the moment the following chips are supported:

  * [nRF52832](https://www.nordicsemi.com/Products/Low-power-short-range-wireless/nRF52832) with the [S132](https://www.nordicsemi.com/Software-and-Tools/Software/S132) SoftDevice (version 6).
  * [nRF52840](https://www.nordicsemi.com/Products/Low-power-short-range-wireless/nRF52840) with the [S140](https://www.nordicsemi.com/Software-and-Tools/Software/S140) SoftDevice (version 6 and 7).
  * [nRF51822](https://www.nordicsemi.com/Products/Low-power-short-range-wireless/nRF51822) with the [S110](https://www.nordicsemi.com/Software-and-Tools/Software/S110) SoftDevice (version 8). This SoftDevice does not support all features (e.g. scanning).

### Adafruit "Bluefruit" boards

The support for boards created by Adafruit already have the Nordic Semi SoftDevice firmware pre-installed. You can use TinyGo with this package without any additional steps required. Supported boards include:

* [Adafruit Circuit Playground Bluefruit](https://www.adafruit.com/product/4333)
* [Adafruit CLUE Alpha](https://www.adafruit.com/product/4500)
* [Adafruit Feather nRF52840 Express](https://www.adafruit.com/product/4062)
* [Adafruit ItsyBitsy nRF52840](https://www.adafruit.com/product/4481)

### Flashing the SoftDevice

Other boards that use supported chips from Nordic Semi that do not already have the SoftDevice firmware must have it installed on the board in order to use this package.

Flashing the SoftDevice can be tricky. If you have [nrfjprog](https://www.nordicsemi.com/Software-and-Tools/Development-Tools/nRF-Command-Line-Tools) installed, you can erase the flash and flash the new BLE firmware using the following commands. Replace the path to the hex file with the correct SoftDevice, for example `s132_nrf52_6.1.1/s132_nrf52_6.1.1_softdevice.hex` for S132 version 6.

    nrfjprog -f nrf52 --eraseall
    nrfjprog -f nrf52 --program path/to/softdevice.hex

After that, don't reset the board but instead flash a new program to it. For example, you can flash the Heart Rate Sensor example using `tinygo` (modify the `-target` flag as needed for your board):

    tinygo flash -target=pca10040-s132v6 ./examples/heartrate

Flashing will normally reset the board.

For boards that use the CMSIS-DAP interface (such as the [BBC micro:bit](https://microbit.org/)), this works a bit different. Flashing the SoftDevice is done by simply copying the .hex file to the device, for example (on Linux):

    cp path/to/softdevice.hex /media/yourusername/MICROBIT/

Flashing will then need to be done a bit differently, using the CMSIS-DAP interface instead of the mass-storage interface normally used by TinyGo:

    tinygo flash -target=microbit-s110v8 -programmer=cmsis-dap ./examples/heartrate

## API stability

**The API is not stable!** Because many features are not yet implemented and some platforms (e.g. Windows and macOS) are not yet fully supported, it's hard to say what a good API will be. Therefore, if you want stability you should pick a particular git commit and use that. Go modules can be useful for this purpose.

Some things that will probably change:

  * Add options to the `Scan` method, for example to filter on UUID.
  * Extra options to the `Enable` function, to request particular features (such as the number of peripheral connections supported).

This package will probably remain unstable until the following has been implemented:

  * Scan filters. For example, to filter on service UUID.
  * Bonding and private addresses.
  * Usable support on at least two desktop operating systems.
  * Maybe some Bluetooth Classic support, such as A2DP.

## Contributing

Your contributions are welcome!

Please take a look at our [CONTRIBUTING.md](./CONTRIBUTING.md) document for details.

## License

This project is licensed under the BSD 3-clause license, see the LICENSE file for details.

The SoftDevices from Nordic are licensed under a different license, check the license file in the SoftDevice source directory.
