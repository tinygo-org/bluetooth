// +build softdevice,s140v7

// This file is necessary to define SVCall wrappers, because TinyGo does not yet
// support static functions in the preamble.

// Discard all 'static' attributes to define functions normally.
#define static

// Get rid of all __STATIC_INLINE symbols.
// This is a bit less straightforward: we first need to include the header that
// defines it, and then redefine it.
#include "nrf.h"
#undef __STATIC_INLINE
#define __STATIC_INLINE

#include "s140_nrf52_7.0.1/s140_nrf52_7.0.1_API/include/nrf_sdm.h"
#include "s140_nrf52_7.0.1/s140_nrf52_7.0.1_API/include/nrf_nvic.h"
#include "s140_nrf52_7.0.1/s140_nrf52_7.0.1_API/include/ble.h"

// Define nrf_nvic_state, which is used by sd_nvic_critical_region_enter and
// sd_nvic_critical_region_exit.
nrf_nvic_state_t nrf_nvic_state = {0};
