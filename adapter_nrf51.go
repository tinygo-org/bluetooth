// +build softdevice,s110v8

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "nrf_sdm.h"
#include "ble.h"
#include "ble_gap.h"

void assertHandler(void);
*/
import "C"

import "unsafe"

//export assertHandler
func assertHandler(pc uint32, line_number uint16, p_file_name *byte) {
	println("SoftDevice assert")
}

func (a *Adapter) enable() error {
	// Enable the SoftDevice.
	errCode := C.sd_softdevice_enable(C.NRF_CLOCK_LFCLKSRC_RC_250_PPM_250MS_CALIBRATION, C.softdevice_assertion_handler_t(C.assertHandler))
	if errCode != 0 {
		return Error(errCode)
	}

	// Enable the BLE stack.
	enableParams := C.ble_enable_params_t{
		gatts_enable_params: C.ble_gatts_enable_params_t{
			attr_tab_size: C.BLE_GATTS_ATTR_TAB_SIZE_DEFAULT,
		},
	}
	errCode = C.sd_ble_enable(&enableParams)
	return makeError(errCode)
}

func handleEvent() {
	id := eventBuf.header.evt_id
	switch {
	case id >= C.BLE_GAP_EVT_BASE && id <= C.BLE_GAP_EVT_LAST:
		gapEvent := eventBuf.evt.unionfield_gap_evt()
		switch id {
		case C.BLE_GAP_EVT_CONNECTED:
			currentConnection.Reg = gapEvent.conn_handle
			DefaultAdapter.connectHandler(nil, true)
		case C.BLE_GAP_EVT_DISCONNECTED:
			if defaultAdvertisement.isAdvertising.Get() != 0 {
				// The advertisement was running but was automatically stopped
				// by the connection event.
				// Note that it cannot be restarted during connect like this,
				// because it would need to be reconfigured as a non-connectable
				// advertisement. That's left as a future addition, if
				// necessary.
				defaultAdvertisement.start()
			}
			currentConnection.Reg = C.BLE_CONN_HANDLE_INVALID
			DefaultAdapter.connectHandler(nil, false)
		case C.BLE_GAP_EVT_CONN_PARAM_UPDATE_REQUEST:
			// Respond with the default PPCP connection parameters by passing
			// nil:
			// > If NULL is provided on a peripheral role, the parameters in the
			// > PPCP characteristic of the GAP service will be used instead. If
			// > NULL is provided on a central role and in response to a
			// > BLE_GAP_EVT_CONN_PARAM_UPDATE_REQUEST, the peripheral request
			// > will be rejected
			C.sd_ble_gap_conn_param_update(gapEvent.conn_handle, nil)
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
		default:
			if debug {
				println("unknown GATTS event:", id, id-C.BLE_GATTS_EVT_BASE)
			}
		}
	default:
		if debug {
			println("unknown event:", id)
		}
	}
}
