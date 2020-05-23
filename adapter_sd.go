// +build softdevice

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

import (
	"device/arm"
	"device/nrf"
	"errors"
	"unsafe"
)

var (
	ErrNotDefaultAdapter = errors.New("bluetooth: not the default adapter")
)

//export assertHandler
func assertHandler() {
	println("SoftDevice assert")
}

var clockConfig C.nrf_clock_lf_cfg_t = C.nrf_clock_lf_cfg_t{
	source:       C.NRF_CLOCK_LF_SRC_SYNTH,
	rc_ctiv:      0,
	rc_temp_ctiv: 0,
	accuracy:     0,
}

var (
	secModeOpen       C.ble_gap_conn_sec_mode_t // No security is needed (aka open link).
	defaultDeviceName = [6]byte{'T', 'i', 'n', 'y', 'G', 'o'}
)

// Globally allocated buffer for incoming SoftDevice events.
var eventBuf struct {
	C.ble_evt_t
	buf [23]byte
}

func init() {
	secModeOpen.set_bitfield_sm(1)
	secModeOpen.set_bitfield_lv(1)
}

// Adapter is a dummy adapter: it represents the connection to the (only)
// SoftDevice on the chip.
type Adapter struct {
	isDefault         bool
	handler           func(Event)
	charWriteHandlers []charWriteHandler
}

// defaultAdapter is an adapter to the default Bluetooth stack on a given
// target.
var defaultAdapter = Adapter{isDefault: true}

// DefaultAdapter returns the default adapter on the current system. On Nordic
// chips, it will return the SoftDevice interface.
func DefaultAdapter() (*Adapter, error) {
	return &defaultAdapter, nil
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	if !a.isDefault {
		return ErrNotDefaultAdapter
	}

	// Enable the IRQ that handles all events.
	arm.EnableIRQ(nrf.IRQ_SWI2)
	arm.SetPriority(nrf.IRQ_SWI2, 192)

	errCode := C.sd_softdevice_enable(&clockConfig, C.nrf_fault_handler_t(C.assertHandler))
	if errCode != 0 {
		return Error(errCode)
	}

	appRAMBase := uint32(0x200039c0)
	errCode = C.sd_ble_enable(&appRAMBase)
	if errCode != 0 {
		return Error(errCode)
	}

	errCode = C.sd_ble_gap_device_name_set(&secModeOpen, &defaultDeviceName[0], uint16(len(defaultDeviceName)))
	if errCode != 0 {
		return Error(errCode)
	}

	var gapConnParams C.ble_gap_conn_params_t
	gapConnParams.min_conn_interval = C.BLE_GAP_CP_MIN_CONN_INTVL_MIN
	gapConnParams.max_conn_interval = C.BLE_GAP_CP_MIN_CONN_INTVL_MAX
	gapConnParams.slave_latency = 0
	gapConnParams.conn_sup_timeout = C.BLE_GAP_CP_CONN_SUP_TIMEOUT_NONE

	errCode = C.sd_ble_gap_ppcp_set(&gapConnParams)
	if errCode != 0 {
		return Error(errCode)
	}

	return nil
}

func handleEvent() {
	id := eventBuf.header.evt_id
	switch {
	case id >= C.BLE_GAP_EVT_BASE && id <= C.BLE_GAP_EVT_LAST:
		handler := defaultAdapter.handler
		if handler == nil {
			return
		}
		gapEvent := GAPEvent{
			Connection: Connection(eventBuf.evt.unionfield_gap_evt().conn_handle),
		}
		switch id {
		case C.BLE_GAP_EVT_CONNECTED:
			handler(&ConnectEvent{GAPEvent: gapEvent})
		case C.BLE_GAP_EVT_DISCONNECTED:
			handler(&DisconnectEvent{GAPEvent: gapEvent})
		}
	case id >= C.BLE_GATTS_EVT_BASE && id <= C.BLE_GATTS_EVT_LAST:
		gattsEvent := eventBuf.evt.unionfield_gatts_evt()
		switch id {
		case C.BLE_GATTS_EVT_WRITE:
			writeEvent := gattsEvent.params.unionfield_write()
			len := writeEvent.len - writeEvent.offset
			data := (*[255]byte)(unsafe.Pointer(&writeEvent.data[0]))[:len:len]
			handler := defaultAdapter.getCharWriteHandler(writeEvent.handle)
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
		}
	}
}

//go:export SWI2_EGU2_IRQHandler
func handleInterrupt() {
	for {
		eventBufLen := uint16(unsafe.Sizeof(eventBuf))
		errCode := C.sd_ble_evt_get((*uint8)(unsafe.Pointer(&eventBuf)), &eventBufLen)
		if errCode != 0 {
			// Possible error conditions:
			//  * NRF_ERROR_NOT_FOUND: no events left, break
			//  * NRF_ERROR_DATA_SIZE: retry with a bigger data buffer
			//    (currently not handled, TODO)
			//  * NRF_ERROR_INVALID_ADDR: pointer is not aligned, should
			//    not happen.
			// In all cases, it's best to simply stop now.
			break
		}
		handleEvent()
	}
}
