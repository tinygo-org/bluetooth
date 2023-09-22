//go:build softdevice

package bluetooth

// #include <stdint.h>
// #include "nrf_error.h"
// #include "nrf_error_sdm.h"
import "C"

// Error is an error from within the SoftDevice.
type Error uint32

func (e Error) Error() string {
	switch {
	case e >= C.NRF_ERROR_BASE_NUM && e < C.NRF_ERROR_SDM_BASE_NUM:
		// Global errors.
		switch e {
		case C.NRF_SUCCESS:
			return "no error"
		case C.NRF_ERROR_SVC_HANDLER_MISSING:
			return "SVC handler is missing"
		case C.NRF_ERROR_SOFTDEVICE_NOT_ENABLED:
			return "SoftDevice has not been enabled"
		case C.NRF_ERROR_INTERNAL:
			return "internal error"
		case C.NRF_ERROR_NO_MEM:
			return "no memory for operation"
		case C.NRF_ERROR_NOT_FOUND:
			return "not found"
		case C.NRF_ERROR_NOT_SUPPORTED:
			return "not supported"
		case C.NRF_ERROR_INVALID_PARAM:
			return "invalid parameter"
		case C.NRF_ERROR_INVALID_STATE:
			return "invalid state, operation disallowed in this state"
		case C.NRF_ERROR_INVALID_LENGTH:
			return "invalid Length"
		case C.NRF_ERROR_INVALID_FLAGS:
			return "invalid flags"
		case C.NRF_ERROR_INVALID_DATA:
			return "invalid data"
		case C.NRF_ERROR_DATA_SIZE:
			return "invalid data size"
		case C.NRF_ERROR_TIMEOUT:
			return "operation timed out"
		case C.NRF_ERROR_NULL:
			return "null pointer"
		case C.NRF_ERROR_FORBIDDEN:
			return "forbidden operation"
		case C.NRF_ERROR_INVALID_ADDR:
			return "bad memory address"
		case C.NRF_ERROR_BUSY:
			return "busy"
		case 18: // C.NRF_ERROR_CONN_COUNT, not available on nrf51
			return "maximum connection count exceeded"
		case 19: // C.NRF_ERROR_RESOURCES, not available on nrf51
			return "not enough resources for operation"
		default:
			return "other global error"
		}
	case e >= C.NRF_ERROR_SDM_BASE_NUM && e < C.NRF_ERROR_SOC_BASE_NUM:
		// SDM errors.
		switch e {
		case C.NRF_ERROR_SDM_LFCLK_SOURCE_UNKNOWN:
			return "unknown LFCLK source"
		case C.NRF_ERROR_SDM_INCORRECT_INTERRUPT_CONFIGURATION:
			return "incorrect interrupt configuration"
		case C.NRF_ERROR_SDM_INCORRECT_CLENR0:
			return "incorrect CLENR0"
		default:
			return "other SDM error"
		}
	case e >= C.NRF_ERROR_SOC_BASE_NUM && e < C.NRF_ERROR_STK_BASE_NUM:
		// SoC errors.
		return "other SoC error"
	case e >= C.NRF_ERROR_STK_BASE_NUM && e < 0x4000:
		// STK errors.
		return "other STK error"
	default:
		// Other errors.
		return "other error"
	}
}

// makeError returns an error (using the Error type) if the error code is
// non-zero, otherwise it returns nil. It is used with internal API calls.
func makeError(code C.uint32_t) error {
	if code != 0 {
		return Error(code)
	}
	return nil
}
