0.12.0
---
* **core**
  - Introduce AdvertisementPayload.ServiceUUIDs()
* **windows**
  - Fix device disconnect for windows (#368)
  - Fix write respond


0.12.0
---
* **core**
  - UUID: implement encoding methods MarshalBinary, UnmarshalBinary, AppendBinary, MarshalText, UnmarshalText, and AppendText. Also enhance testing and improve performance. ParseUUID and UUID.String now use these methods. UnmarshalText supports 128, 32, and 16-bit UUIDs.
  - MAC: implement encoding interfaces (TextMarshaler, BinaryMarshaler, TextAppender, BinaryAppender) for efficient encoding and use with packages like encoding/json and encoding/xml. String method performance improved.
  - fix: correctly handle slice of connection handles for notification updates.
* **hci**
  - fix: handle l2cap connection param req updates with invalid length data.
* **linux**
  - Fix: `Adapter.connectHandler` is now called after `Advertisement.Start`. D-Bus signals added to handle connect/disconnect events when peripheral is advertising (#369).
  - Fix: ensure `connectHandler` is invoked on initial device connection.
  - Advertising: Add alias update on advertising.
  - Advertising: Extend property handling for D-Bus signals.
  - Adapter: check powered state before connecting.
  - Adapter: call `startDiscovery` asynchronously as it can block if adapter is powered off.
  - Adapter: close scan-in-progress channel if adapter is powered off during scan.
  - Adapter: handle adapter power state to return an error if the adapter is powered off while connecting.
  - Adapter: handle adapter power state to return an error if the adapter is powered off while scanning.
* **windows**
  - Fix: Notify/Indicate characteristics detection.
* **modules**
  - Update to latest `soypat/cyw43439` package.
  - Update to use `cyw43439` package with BLE read length fix.
* **docs**
  - Update inline documentation for Linux `Adapter.SetConnectHandler`.
  - Update README usage example for Linux `connectHandler`.
  - Correct README example for Linux advertising.
  - Add info on flashing the Microbit v2.
* **examples**
  - Add resource cleanup in Linux advertising example.
  - Add `connectHandler` to Linux advertisement example.


0.11.0
---
* **core**
  - gap: do not add ADFlags to advertising data for AdvertisingTypeNonConnInd, to leave room for FindMy data
  - gap: make generic services function for HCI separate function
  - update to latest version of Bluetooth numbers database for latest services/characteristics
* **linux**
  - gap: add implementation for SetRandomAddress() function
  - gap: correct HCI implementation so it can Configure/Start/Stop advertising correctly. Needed to update ServiceData after Advertisment is already started.
  - gap: update implementation for GAP to allow for calling adv.Configure() followed by adv.Start() followed by adv.Stop() multiple times. This is required in order to update advertising ServiceData.
  - fix: close signal on Connect for Linux platform to address issues raised in #262
  - reconnect connect/disconnect handlers centrals
  - Allow users to choose between different adapters
  - Issue #311 Clean resources properly when disable notifications
* **hci**
  - add advertiser support for ManufacturerData and ServiceData
  - contains several corrections needed for HCI on both ninafw and cyw43439
  - implement set random MAC address
  - store local address as MACAddress
  - use current MAC address when Advertiser, allowing support for random advertiser addresses
  - when advertising, do not send ADFlags if we are using AdvertisingTypeNonConnInd
  - implement connect handler for Advertiser for HCI
  - fix bluetooth initialization on cyw43439
  - actually parse of ServiceData and ManufacturerData when scanning, and use existing GAP implementation to do it
  - correct parsing of ServiceData and ManufacturerData and add some helpful constants
  - correct race condition from connection params in HCI implementation. Basically it was responding by calling a function that retriggered asking to change the parameters yet again. Yet we are not actually doing anything about this request at present.
  - ensure HCI send/receive buffers for ATT are large enough for maximum MTU length
  - ensure that HCI advertising interval is set to a default value, otherwise different adverting types do not work correctly
  - reconnect connect/disconnect handlers for centrals
  - fix: set HCI default Advertising Interval to a more sensible default
  - enable setting MACAddr for cyw43439
* **nordic semi**
  - nrf51: add SetRandomAddress() function when Advertisment
  - nrf528xx: add SetRandomAddress() function when Advertiser
  - nrf528xx: correctly use the passed Advertising options type as the type passed into the SoftDevice struct for API call
update the EnableNotifications Method, add the parameter to specifing… (#293)
* **windows**
  - gap/windows: add stubbed function for SetRandomAddress()
  - reconnect connect/disconnect handlers for centrals
* **examples**
  - add example showing how to create a peripheral using the Battery Service
  - add example showing how to create a peripheral using the Device Information Service
  - add example to broadcast advertising servicedata
  - improve heartrate examples by adding all of the required characteristics for it to fulfill the complete heart rate profile in the spec.
  - refactor tinyscan to use tinyterm/displays package and also add badger2040-w
* **build**
  - remove macOS 11 build since it has been removed, and run macOS 14 instead (#285)
  - remove macOS 12 runner
* **docs**
  - update README to include Windows CI badge and correctly updated info about current capabilities
  - update README with some links and clarifications
  - license: update year to 2025


0.10.0
---

* **core**
  - gap: fix ServiceDataElement.UUID comment
* **docs**
  - add mention of support for rp2040-W to README
  - Improve documentation of RSSI Fixes https://github.com/tinygo-org/bluetooth/issues/272
* **hci**
  - cyw43439: HCI implementation
  - refactor to separate HCI transport implementation from interface to not always assume UART.
  - update for cyw43439 HCI functionality
* **windows**
  - Add Address field to Windows Device struct
  - Winrt full support (#266)
  - winrt-go: bump to latest
  - assign char handle write event (#274)
* **test**
  - add hci_uart based implementation to smoke tests

0.9.0
---

* **build**
  - add arduino-nano33 and pyportal to smoke tests
  - add nina-fw smoketest as peripheral
  - add some ninafw examples to smoketest
* **core**
  - add ServiceData advertising element (#243)
  - add RequestConnectionParams to request new connection parameters
  - change ManufacturerData from a map to a slice
  - don't use a pointer receiver for many method calls
  - make Device a value instead of a pointer
  - use 'debug' variable protected by build tags for debug logging
  - use Device instead of Address in SetConnectHandler
* **docs**
  - a small mention of the NINA BLE support
  - complete README info about nina-fw support
* **linux**
  - fix characteristic value
  - rewrite everything to use DBus directly
* **macos**
  - add Write command to the gattc implementation
* **examples**
  - tinyscan to replace clue-scanner, also works on pyportal and pybadge+airlift
  - update MCU central examples to use ldflags to pass the desired device to connect to
  - discover: add MTU
* **hci**
  - add check for poll buffer overflow
  - allow for both ninafw and pure hci uart adapter implementations
  - implement Characteristic WriteHandler
  - multiple connections
  - return service UUIDs with scan results
  - add l2cap signaling support
  - implement evtNumCompPkts to count in-flight packets
  - correct implementation for WriteWithoutReponse
  - speed up time waiting for hardware - corrections to MTU exchange
  - add support for software RTS/CTS flow control for boards where hardware support is not available
  - BLE central implementation on nina-fw co-processors
  - fix connection timeout
  - implement BLE peripheral support
  - implement GetMTU()
  - remove some pointer receivers from method calls
  - should support muliple connections as a central
  - correctly return from read requests instead of returning spurious error
  - move some steps previously being done during Configure() into Start() where they more correctly belonged.
  - use advertising display name as the correct default value for the generic access characteristic.
  - speed up the polling for new notifications for Centrals
  - use NINA settings from board file in main TinyGo repo
* **nordic semi**
  - replace unsafe.SliceData call with expression that is still supported in older Go versions
  - update to prepare for changes in the TinyGo CGo implementation
  - add address of connecting device
  - add support for connection timeout on connect
  - don't send a notify/indicate without a CCCD
  - fix connect timeout
  - fix writing to a characteristic
  - print connection parameters when debug is enabled
  - return an error on a connection timeout
* **windows**
  - Release AsyncOperationCompletedHandler (#208)
  - check for error when scanning
  - bump to latest winrt


0.8.0
---

* **build**
  - remove CGo dependencies for Windows cross-compiler tests
  - add Windows to GH actions build jobs
  - add macOS 12 to GH actions build jobs
* **core**
  - go 1.18 and remove old-style build tags
  - Noescape workaround
* **docs**
  - update README to remove CGo requirement for Windows
  - add documentation to heartrate-monitor
* **linux**
  - Added option to add ManufacturerData to Advertisement
* **macos**
  - enable support for duplicate chars by moving from a map to a slice
* **examples**
  - Include WriteWithoutResponse permission, for examples, where Write exists
* **nordic semi**
  - softdevice: added manufacturer data support
  - softdevice: test creation of raw BLE advertisement packets
* **windows**
  - update github.com/saltosystems/winrt-go to no longer require CGo


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
