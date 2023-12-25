//go:build softdevice

package bluetooth

import (
	"device/nrf"
	"errors"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

// #include "ble.h"
// #ifdef NRF51
//   #include "nrf_soc.h"
// #else
//   #include "nrf_nvic.h"
// #endif
import "C"

var (
	ErrNotDefaultAdapter = errors.New("bluetooth: not the default adapter")
)

var (
	secModeOpen       C.ble_gap_conn_sec_mode_t // No security is needed (aka open link).
	defaultDeviceName = [6]byte{'T', 'i', 'n', 'y', 'G', 'o'}
)

// There can only be one connection at a time in the default configuration.
var currentConnection = volatileHandle{handle: volatile.Register16{C.BLE_CONN_HANDLE_INVALID}}

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
	scanning          bool
	charWriteHandlers []charWriteHandler

	connectHandler func(device Device, connected bool)
}

// DefaultAdapter is the default adapter on the current system. On Nordic chips,
// it will return the SoftDevice interface.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{isDefault: true,
	connectHandler: func(device Device, connected bool) {
		return
	}}

var eventBufLen C.uint16_t

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	if !a.isDefault {
		return ErrNotDefaultAdapter
	}

	// Enable the IRQ that handles all events.
	intr := interrupt.New(nrf.IRQ_SWI2, func(interrupt.Interrupt) {
		for {
			eventBufLen = C.uint16_t(unsafe.Sizeof(eventBuf))
			errCode := C.sd_ble_evt_get((*C.uint8_t)(unsafe.Pointer(&eventBuf)), &eventBufLen)
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
	})
	intr.Enable()
	intr.SetPriority(192)

	// Do more specific initialization of this SoftDevice (split out for nrf52*
	// and nrf51 chips because of the different API).
	err := a.enable()
	if err != nil {
		return err
	}

	errCode := C.sd_ble_gap_device_name_set(&secModeOpen, (*C.uint8_t)(unsafe.Pointer(&defaultDeviceName[0])), C.uint16_t(len(defaultDeviceName)))
	if errCode != 0 {
		return Error(errCode)
	}

	var gapConnParams C.ble_gap_conn_params_t
	gapConnParams.min_conn_interval = C.BLE_GAP_CP_MIN_CONN_INTVL_MIN
	gapConnParams.max_conn_interval = C.BLE_GAP_CP_MIN_CONN_INTVL_MAX
	gapConnParams.slave_latency = 0
	gapConnParams.conn_sup_timeout = C.BLE_GAP_CP_CONN_SUP_TIMEOUT_NONE

	errCode = C.sd_ble_gap_ppcp_set(&gapConnParams)
	return makeError(errCode)
}

// DisableInterrupts must be used instead of disabling interrupts directly, to
// play well with the SoftDevice. Restore interrupts to the previous state with
// RestoreInterrupts.
func DisableInterrupts() uintptr {
	var is_nested_critical_region C.uint8_t
	C.sd_nvic_critical_region_enter(&is_nested_critical_region)
	return uintptr(is_nested_critical_region)
}

// RestoreInterrupts restores interrupts to the state before calling
// DisableInterrupts. The mask parameter must be the value returned by
// DisableInterrupts.
func RestoreInterrupts(mask uintptr) {
	C.sd_nvic_critical_region_exit(C.uint8_t(mask))
}

// Wrapper for volatile.Register16 that uses C.uint16_t instead of uint16, for
// easier interoperability with C.
type volatileHandle struct {
	handle volatile.Register16
}

func (a *volatileHandle) Set(handle C.uint16_t) {
	a.handle.Set(uint16(handle))
}

func (a *volatileHandle) Get() C.uint16_t {
	return C.uint16_t(a.handle.Get())
}

// Convert a SoftDevice MAC address into a Go MAC address.
func makeAddress(mac [6]C.uint8_t) MAC {
	return MAC{
		uint8(mac[0]),
		uint8(mac[1]),
		uint8(mac[2]),
		uint8(mac[3]),
		uint8(mac[4]),
		uint8(mac[5]),
	}
}

// Convert a Go MAC address into a SoftDevice MAC Address.
func makeSDAddress(mac MAC) [6]C.uint8_t {
	return [6]C.uint8_t{
		C.uint8_t(mac[0]),
		C.uint8_t(mac[1]),
		C.uint8_t(mac[2]),
		C.uint8_t(mac[3]),
		C.uint8_t(mac[4]),
		C.uint8_t(mac[5]),
	}
}
