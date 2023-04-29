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
