0.7.0
---

* **build**
  - switch to ghcr.io for docker container
  - update to actions/checkout@v3
  - work around for CVE-2022-24765
* **core**
  - gap: Set and SetRandom methods should have a pointer receiver
  - mtu-{darwin,linux,windows,sd}: add get mtu function
  - remove Addresser
  - update uuid generation
* **docs**
  - CONTRIBUTING: add note on new APIs
  - correct badge link for GH actions
  - README: add note on macOS Big Sur and iTerm2
* **linux**
  - do not randomize order of returned discovered services/chars
  - fix characteristic scan order
  - implement disconnect handling
* **macos**
  - implement disconnect handling
  - fix characteristic scan order
* **examples**
  - add examples/stop-advertisement
* **nordic semi**
  - nrf528xx: handle BLE_GAP_EVT_PHY_UPDATE_REQUEST and explicitly ignore some other events
  - softdevice: avoid a heap allocation in the SoftDevice event handler
* **windows**
  - Added Indicate support to Windows driver
  - gap/windows: Scan should set scanning mode to active to match other platforms
  - support empty manufacturer data
  - winrt-go: bump to latest


0.6.0
---
* **core**
  - unify UUID16 creation for all platforms
  - Improve UUID (#107)
  - gap: stop advertising
  - advertising: add manufacturer data field to advertisement payload
* **linux**
  - gap: workaround for https://github.com/muka/go-bluetooth/issues/163
  - update to latest muka/go-bluetooth
* **windows**
  - add characteristic read, write and notify operations
  - add characteristic discovery
  - add service discovery
  - add device connection and disconnection
  - add winrt-go dependency and remove manually generated code
  - disable cache when reading characteristics
* **macos**
  - update to tinygo-org fork of cbgo v0.0.4
  - use the same UUID format as expected by other standard
* **docs**
  - update README with info on Windows support
* **build**
  - add Github Action based CI build (#108)


0.5.0
---
* **core**
  - update to drivers 0.20.0
  - Fix ParseMAC bug
  - Add //go:build lines for Go 1.18
* **nordic semi**
  - nrf: fix CGo errors after TinyGo update

0.4.0
---
* **core**
  - adapter: add host address function
* **linux**
  - fixes bluez 0.55 service registration
  - update muka/go-bluetooth to latest version
  - gattc/linux: DiscoverServices times out in 10s
* **macos**
  - make Adapter.Connect thread-safe
* **nordic semi**
  - nrf51: fix assertHandler function signature
  - nrf: add support for S113 SoftDevice
  - nrf: update s140v7 SoftDevice version to latest, 7.3.0
* **examples**
  - add scanner for Adafruit Clue
* **build**
  - circleci: update xcode in use to 10.3.0
  - modules: add tinyterm package for clue example

0.3.0
---
* **core**
  - generate standard service and characteristic UUIDs from Nordic Semiconductor bluetooth numbers database
* **linux**
  - downgrade to older version of go-bluetooth that appears to work correctly with BlueZ 5.50
* **macos**
  - properly handle 16-bit UUIDs for service and characteristics in the unique format used by macOS
* **docs**
  - add a few details on some newly supported boards
* **examples**
  - use standard service and characteristic UUIDs
  - correct heart rate monitor data format

0.2.0
---
* **core**
  - gattc: DeviceCharacteristic Read() implementation
  - gap: add Disconnect() to Driver
  - gap: change signature for Addresser interface Set() function to accept string and then parse it as needed
* **linux**
  - update to latest version of go-bluetooth package for Linux
* **macos**
  - handle case when adapter enable sends notification before event delegate is set
  - Document async Disconnect behaviour
* **examples**
  - discover should only wait on startup on baremetal, since macOS does not like that

0.1.0
---
* **linux**
  - support for both central and peripheral operation
* **macos**
  - support for both central and peripheral operation
* **windows**
  - experimental support for both central scanning only
* **nordic semiconductor**
  - support for both central and peripheral operation on nRF82840 and nRF52832
  - support for peripheral only on nRF51822
