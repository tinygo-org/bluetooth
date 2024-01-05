//go:build (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

// This file implements the event handler for SoftDevices with full support:
// both central and peripheral mode. This includes S132 and S140.

/*
#include "nrf_sdm.h"
#include "nrf_nvic.h"
#include "ble.h"
#include "ble_gap.h"
*/
import "C"

import (
	"unsafe"
)

func handleEvent() {
	id := eventBuf.header.evt_id
	switch {
	case id >= C.BLE_GAP_EVT_BASE && id <= C.BLE_GAP_EVT_LAST:
		gapEvent := eventBuf.evt.unionfield_gap_evt()
		switch id {
		case C.BLE_GAP_EVT_CONNECTED:
			connectEvent := gapEvent.params.unionfield_connected()
			device := Device{
				Address:          Address{makeMACAddress(connectEvent.peer_addr)},
				connectionHandle: gapEvent.conn_handle,
			}
			switch connectEvent.role {
			case C.BLE_GAP_ROLE_PERIPH:
				if debug {
					println("evt: connected in peripheral role")
				}
				currentConnection.handle.Reg = uint16(gapEvent.conn_handle)
				DefaultAdapter.connectHandler(device, true)
			case C.BLE_GAP_ROLE_CENTRAL:
				if debug {
					println("evt: connected in central role")
				}
				connectionAttempt.connectionHandle = gapEvent.conn_handle
				connectionAttempt.state.Set(2) // connection was successful
				DefaultAdapter.connectHandler(device, true)
			}
		case C.BLE_GAP_EVT_DISCONNECTED:
			if debug {
				println("evt: disconnected")
			}
			// Clean up state for this connection.
			for i, cb := range gattcNotificationCallbacks {
				if uint16(cb.connectionHandle) == currentConnection.handle.Reg {
					gattcNotificationCallbacks[i].valueHandle = 0 // 0 means invalid
				}
			}
			currentConnection.handle.Reg = C.BLE_CONN_HANDLE_INVALID
			// Auto-restart advertisement if needed.
			if defaultAdvertisement.isAdvertising.Get() != 0 {
				// The advertisement was running but was automatically stopped
				// by the connection event.
				// Note that it cannot be restarted during connect like this,
				// because it would need to be reconfigured as a non-connectable
				// advertisement. That's left as a future addition, if
				// necessary.
				C.sd_ble_gap_adv_start(defaultAdvertisement.handle, C.BLE_CONN_CFG_TAG_DEFAULT)
			}
			device := Device{
				connectionHandle: gapEvent.conn_handle,
			}
			DefaultAdapter.connectHandler(device, false)
		case C.BLE_GAP_EVT_CONN_PARAM_UPDATE:
			if debug {
				// Print connection parameters for easy debugging.
				params := gapEvent.params.unionfield_conn_param_update().conn_params
				interval_ms := params.min_conn_interval * 125 / 100 // min and max are the same here
				print("conn param update interval=", interval_ms, "ms latency=", params.slave_latency, " timeout=", params.conn_sup_timeout*10, "ms")
				println()
			}
		case C.BLE_GAP_EVT_ADV_REPORT:
			advReport := gapEvent.params.unionfield_adv_report()
			if debug && &scanReportBuffer.data[0] != (*byte)(unsafe.Pointer(advReport.data.p_data)) {
				// Sanity check.
				panic("scanReportBuffer != advReport.p_data")
			}
			// Prepare the globalScanResult, which will be passed to the
			// callback.
			scanReportBuffer.len = byte(advReport.data.len)
			globalScanResult.RSSI = int16(advReport.rssi)
			globalScanResult.Address = Address{
				makeMACAddress(advReport.peer_addr),
			}
			globalScanResult.AdvertisementPayload = &scanReportBuffer
			// Signal to the main thread that there was a scan report.
			// Scanning will be resumed (from the main thread) once the scan
			// report has been processed.
			gotScanReport.Set(1)
		case C.BLE_GAP_EVT_CONN_PARAM_UPDATE_REQUEST:
			// Respond with the default PPCP connection parameters by passing
			// nil:
			// > If NULL is provided on a peripheral role, the parameters in the
			// > PPCP characteristic of the GAP service will be used instead. If
			// > NULL is provided on a central role and in response to a
			// > BLE_GAP_EVT_CONN_PARAM_UPDATE_REQUEST, the peripheral request
			// > will be rejected
			C.sd_ble_gap_conn_param_update(gapEvent.conn_handle, nil)
		case C.BLE_GAP_EVT_DATA_LENGTH_UPDATE_REQUEST:
			// We need to respond with sd_ble_gap_data_length_update. Setting
			// both parameters to nil will make sure we send the default values.
			C.sd_ble_gap_data_length_update(gapEvent.conn_handle, nil, nil)
		case C.BLE_GAP_EVT_DATA_LENGTH_UPDATE:
			// ignore confirmation of data length successfully updated
		case C.BLE_GAP_EVT_PHY_UPDATE_REQUEST:
			phyUpdateRequest := gapEvent.params.unionfield_phy_update_request()
			C.sd_ble_gap_phy_update(gapEvent.conn_handle, &phyUpdateRequest.peer_preferred_phys)
		case C.BLE_GAP_EVT_PHY_UPDATE:
			// ignore confirmation of phy successfully updated
		case C.BLE_GAP_EVT_TIMEOUT:
			timeoutEvt := gapEvent.params.unionfield_timeout()
			switch timeoutEvt.src {
			case C.BLE_GAP_TIMEOUT_SRC_CONN:
				// Failed to connect to a peripheral.
				if debug {
					println("gap timeout: conn")
				}
				connectionAttempt.state.Set(3) // connection timed out
			default:
				// For example a scan timeout.
				if debug {
					println("gap timeout: other")
				}
			}
		default:
			if debug {
				println("unknown GAP event:", id)
			}
		}
	case id >= C.BLE_GATTS_EVT_BASE && id <= C.BLE_GATTS_EVT_LAST:
		gattsEvent := eventBuf.evt.unionfield_gatts_evt()
		switch id {
		case C.BLE_GATTS_EVT_WRITE:
			writeEvent := gattsEvent.params.unionfield_write()
			len := writeEvent.len - writeEvent.offset
			data := (*[255]byte)(unsafe.Pointer(&writeEvent.data[0]))[:len:len]
			handler := DefaultAdapter.getCharWriteHandler(writeEvent.handle)
			if handler != nil {
				handler.callback(Connection(gattsEvent.conn_handle), int(writeEvent.offset), data)
			}
		case C.BLE_GATTS_EVT_SYS_ATTR_MISSING:
			// This event is generated when reading the Generic Attribute
			// service. It appears to be necessary for bonded devices.
			// From the docs:
			// > If the pointer is NULL, the system attribute info is
			// > initialized, assuming that the application does not have any
			// > previously saved system attribute data for this device.
			// Maybe we should look at the error, but as there's not really a
			// way to handle it, ignore it.
			C.sd_ble_gatts_sys_attr_set(gattsEvent.conn_handle, nil, 0, 0)
		case C.BLE_GATTS_EVT_EXCHANGE_MTU_REQUEST:
			// This event is generated by some devices. While we could support
			// larger MTUs, this default MTU is supported everywhere.
			C.sd_ble_gatts_exchange_mtu_reply(gattsEvent.conn_handle, C.BLE_GATT_ATT_MTU_DEFAULT)
		case C.BLE_GATTS_EVT_HVN_TX_COMPLETE:
			// ignore confirmation of a notification successfully sent
		default:
			if debug {
				println("unknown GATTS event:", id, id-C.BLE_GATTS_EVT_BASE)
			}
		}
	case id >= C.BLE_GATTC_EVT_BASE && id <= C.BLE_GATTC_EVT_LAST:
		gattcEvent := eventBuf.evt.unionfield_gattc_evt()
		switch id {
		case C.BLE_GATTC_EVT_PRIM_SRVC_DISC_RSP:
			discoveryEvent := gattcEvent.params.unionfield_prim_srvc_disc_rsp()
			if debug {
				println("evt: discovered primary service", discoveryEvent.count)
			}
			discoveringService.state.Set(2) // signal there is a result
			if discoveryEvent.count >= 1 {
				// Theoretically there may be more, but as we're only using
				// sd_ble_gattc_primary_services_discover, there should only be
				// one discovered service. Use the first as a sensible fallback.
				discoveringService.startHandle.Set(discoveryEvent.services[0].handle_range.start_handle)
				discoveringService.endHandle.Set(discoveryEvent.services[0].handle_range.end_handle)
				discoveringService.uuid = discoveryEvent.services[0].uuid
			} else {
				// No service found.
				discoveringService.startHandle.Set(0)
			}
		case C.BLE_GATTC_EVT_CHAR_DISC_RSP:
			discoveryEvent := gattcEvent.params.unionfield_char_disc_rsp()
			if debug {
				println("evt: discovered characteristics", discoveryEvent.count)
			}
			if discoveryEvent.count >= 1 {
				// There may be more, but for ease of implementing we only
				// handle the first.
				discoveringCharacteristic.handle_value.Set(discoveryEvent.chars[0].handle_value)
				discoveringCharacteristic.char_props = discoveryEvent.chars[0].char_props
				discoveringCharacteristic.uuid = discoveryEvent.chars[0].uuid
			} else {
				// zero indicates we received no characteristic, set handle_value to last
				discoveringCharacteristic.handle_value.Set(0xffff)
			}
		case C.BLE_GATTC_EVT_DESC_DISC_RSP:
			discoveryEvent := gattcEvent.params.unionfield_desc_disc_rsp()
			if debug {
				println("evt: discovered descriptors", discoveryEvent.count)
			}
			if discoveryEvent.count >= 1 {
				// There may be more, but for ease of implementing we only
				// handle the first.
				uuid := discoveryEvent.descs[0].uuid
				if uuid._type == C.BLE_UUID_TYPE_BLE && uuid.uuid == 0x2902 {
					// Found a CCCD (Client Characteristic Configuration
					// Descriptor), which has a 16-bit UUID with value 0x2902).
					discoveringCharacteristic.handle_value.Set(discoveryEvent.descs[0].handle)
				} else {
					// Found something else?
					// TODO: handle this properly by continuing the scan. For
					// now, give up if we found something other than a CCCD.
					if debug {
						println("  found some other descriptor (unimplemented)")
					}
				}
			}
		case C.BLE_GATTC_EVT_READ_RSP:
			readEvent := gattcEvent.params.unionfield_read_rsp()
			if debug {
				println("evt: read response, data length", readEvent.len)
			}
			readingCharacteristic.handle_value.Set(readEvent.handle)
			readingCharacteristic.offset = readEvent.offset
			readingCharacteristic.length = readEvent.len

			// copy read event data into Go slice
			copy(readingCharacteristic.value, (*[255]byte)(unsafe.Pointer(&readEvent.data[0]))[:readEvent.len:readEvent.len])
		case C.BLE_GATTC_EVT_HVX:
			hvxEvent := gattcEvent.params.unionfield_hvx()
			switch hvxEvent._type {
			case C.BLE_GATT_HVX_NOTIFICATION:
				if debug {
					println("evt: notification", hvxEvent.handle)
				}
				// Find the callback and call it (if there is any).
				for _, callbackInfo := range gattcNotificationCallbacks {
					if callbackInfo.valueHandle == hvxEvent.handle && callbackInfo.connectionHandle == gattcEvent.conn_handle {
						// Create a Go slice from the data, to pass to the
						// callback.
						data := (*[255]byte)(unsafe.Pointer(&hvxEvent.data[0]))[:hvxEvent.len:hvxEvent.len]
						if callbackInfo.callback != nil {
							callbackInfo.callback(data)
						}
						break
					}
				}
			}
		default:
			if debug {
				println("unknown GATTC event:", id, id-C.BLE_GATTC_EVT_BASE)
			}
		}
	default:
		if debug {
			println("unknown event:", id)
		}
	}
}
