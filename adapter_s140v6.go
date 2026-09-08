//go:build softdevice && s140v6

package bluetooth

/*
// Add the correct SoftDevice include path to CFLAGS, so #include will work as
// expected.
#cgo CFLAGS: -Is140_nrf52_6.1.1/s140_nrf52_6.1.1_API/include

#include "nrf_nvic.h"
#include "nrf_soc.h"
nrf_nvic_state_t nrf_nvic_state = {0};
*/
import "C"

// Transmit power levels in dBm that the nRF52840 accepts.
// See nRF52840 Product Specification v1.11 section 6.20.14.19, RADIO TXPOWER.
var txPowerLevels = []int8{-40, -20, -16, -12, -8, -4, 0, 2, 3, 4, 5, 6, 7, 8}

// setDCSupplyHighVoltage controls the DC/DC converter of the REG0 stage.
// See nRF52840 Product Specification v1.11 section 5.4.1, REG0 stage.
func setDCSupplyHighVoltage(enable bool) error {
	mode := C.uint8_t(C.NRF_POWER_DCDC_DISABLE)
	if enable {
		mode = C.uint8_t(C.NRF_POWER_DCDC_ENABLE)
	}
	return makeError(C.sd_power_dcdc0_mode_set(mode))
}
