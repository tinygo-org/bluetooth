# Go Bluetooth

Bluetooth API for embedded devices.

This package attempts to build a cross-system Bluetooth API written in Go. It
specifically targets embedded devices that are supported by
[TinyGo](https://tinygo.org/).

At the moment, there is only support for the
[S132](https://www.nordicsemi.com/Software-and-Tools/Software/S132)
SoftDevice (binary driver) on Nordic Semiconductors devices.

## Flashing the SoftDevice

Flashing the SoftDevice can be tricky. If you have
[nrfjprog](https://www.nordicsemi.com/Software-and-Tools/Development-Tools/nRF-Command-Line-Tools)
installed, you can erase the flash and flash the new BLE firmware using the
following commands.

    nrfjprog -f nrf52 --eraseall
    nrfjprog -f nrf52 --program s132_nrf52_6.1.1/s132_nrf52_6.1.1_softdevice.hex

After that, don't reset the board but instead flash a new program to it. For
example, you can flash the Heart Rate Sensor example using `tinygo`:

    tinygo flash -target=pca10040-s132v6 ./examples/heartrate
