# Go Bluetooth

[![CircleCI](https://circleci.com/gh/tinygo-org/bluetooth/tree/master.svg?style=svg)](https://circleci.com/gh/tinygo-org/bluetooth/tree/master)
[![GoDoc](https://godoc.org/github.com/tinygo-org/bluetooth?status.svg)](https://godoc.org/github.com/tinygo-org/bluetooth)

This package attempts to build a cross-platform Bluetooth Low Energy module for Go. It currently supports the following systems:

|                       | Windows            | Linux              | Nordic chips       |
| --------------------- | ------------------ | ------------------ | ------------------ |
| API used              | WinRT              | BlueZ (over D-Bus) | SoftDevice         |
| Scanning              | :heavy_check_mark: | :heavy_check_mark: | :heavy_check_mark: |
| Advertisement         | :x:                | :heavy_check_mark: | :heavy_check_mark: |
| Local services        | :x:                | :heavy_check_mark: | :heavy_check_mark: |
| Local characteristics | :x:                | :x:                | :heavy_check_mark: |

## Baremetal support

As you can see above, there is support for some chips from Nordic Semiconductors. At the moment the following chips are supported:

  * The [nRF52832](https://www.nordicsemi.com/Products/Low-power-short-range-wireless/nRF52832) with the [S132](https://www.nordicsemi.com/Software-and-Tools/Software/S132) SoftDevice (version 6).
  * The [nRF52840](https://www.nordicsemi.com/Products/Low-power-short-range-wireless/nRF52840) with the [S140](https://www.nordicsemi.com/Software-and-Tools/Software/S140) SoftDevice (version 7).
  * The [nRF51822](https://www.nordicsemi.com/Products/Low-power-short-range-wireless/nRF51822) with the [S110](https://www.nordicsemi.com/Software-and-Tools/Software/S110) SoftDevice (version 8). This SoftDevice does not support all features (e.g. scanning).

These chips are supported through [TinyGo](https://tinygo.org/).

The SoftDevice is a binary blob that implements the BLE stack. There are other (open source) BLE stacks, but the SoftDevices are pretty solid and have all the qualifications you might need. Other BLE stacks might be added in the future.

### Flashing the SoftDevice

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

**The API is not stable!** Because many features are not yet implemented and some platforms (e.g. MacOS) are not yet supported, it's hard to say what a good API will be. Therefore, if you want stability you should pick a particular git commit and use that. Go modules can be useful for this purpose.

Some things that will probably change:

  * Add options to the `Scan` method, for example to filter on UUID.

## License

This project is licensed under the BSD 3-clause license, see the LICENSE file for details.

The SoftDevices from Nordic are licensed under a different license, check the license file in the SoftDevice source directory.
