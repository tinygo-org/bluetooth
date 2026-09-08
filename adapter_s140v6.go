//go:build softdevice && s140v6

package bluetooth

/*
// Add the correct SoftDevice include path to CFLAGS, so #include will work as
// expected.
#cgo CFLAGS: -Is140_nrf52_6.1.1/s140_nrf52_6.1.1_API/include

#include "nrf_nvic.h"
nrf_nvic_state_t nrf_nvic_state = {0};
*/
import "C"

// Transmit power levels in dBm that the nRF52840 accepts.
// See nRF52840 Product Specification v1.11 section 6.20.14.19, RADIO TXPOWER.
var txPowerLevels = []int8{-40, -20, -16, -12, -8, -4, 0, 2, 3, 4, 5, 6, 7, 8}
