// +build softdevice,s132v6

// This file is necessary to define SVCall wrappers, because TinyGo does not yet
// support static functions in the preamble.

// Discard all 'static' attributes to define functions normally.
#define static

#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/nrf_sdm.h"
#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/ble.h"
