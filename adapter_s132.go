//go:build softdevice && s132v6

package bluetooth

/*
// Add the correct SoftDevice include path to CFLAGS, so #include will work as
// expected.
#cgo CFLAGS: -Is132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include

#include "nrf_nvic.h"
nrf_nvic_state_t nrf_nvic_state = {0};
*/
import "C"

// Transmit power levels in dBm that every part with this SoftDevice accepts.
// See the sd_ble_gap_tx_power_set note in ble_gap.h.
var txPowerLevels = []int8{-40, -20, -16, -12, -8, -4, 0, 3, 4}

// setDCSupplyHighVoltage controls the DC/DC converter of the REG0 stage. Only
// the nRF52840 has a VDDH stage, so this is not supported.
func setDCSupplyHighVoltage(enable bool) error {
	return errNotSupported
}
