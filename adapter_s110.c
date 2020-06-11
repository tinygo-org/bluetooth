// +build softdevice,s110v8

// This file is necessary to define SVCall wrappers, because TinyGo does not yet
// support static functions in the preamble.

// Discard all 'static' attributes to define functions normally.
#define static

#include "s110_nrf51_8.0.0/s110_nrf51_8.0.0_API/include/nrf_sdm.h"
#include "s110_nrf51_8.0.0/s110_nrf51_8.0.0_API/include/nrf_soc.h"
#include "s110_nrf51_8.0.0/s110_nrf51_8.0.0_API/include/ble.h"
