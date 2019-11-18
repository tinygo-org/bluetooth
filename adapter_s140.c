// +build softdevice,s140v7

// This file is necessary to define SVCall wrappers, because TinyGo does not yet
// support static functions in the preamble.

// Discard all 'static' attributes to define functions normally.
#define static

#include "s140_nrf52_7.0.1/s140_nrf52_7.0.1_API/include/nrf_sdm.h"
#include "s140_nrf52_7.0.1/s140_nrf52_7.0.1_API/include/ble.h"
