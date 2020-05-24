// +build softdevice

package bluetooth

import (
	"device/nrf"
	"errors"
	"runtime/interrupt"
	"unsafe"
)

var (
	ErrNotDefaultAdapter = errors.New("bluetooth: not the default adapter")
)

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
	scanning          bool
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
	intr := interrupt.New(nrf.IRQ_SWI2, func(interrupt.Interrupt) {
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
	})
	intr.Enable()
	intr.SetPriority(192)

	// Do more specific initialization of this SoftDevice (split out for nrf52*
	// and nrf51 chips because of the different API).
	err := a.enable()
	if err != nil {
		return err
	}

	errCode := C.sd_ble_gap_device_name_set(&secModeOpen, &defaultDeviceName[0], uint16(len(defaultDeviceName)))
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
