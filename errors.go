package bluetooth

import "fmt"

// AttributeProtocolError represents an error from the Attribute Protocol (ATT)
// as defined in the Bluetooth Core Specification, Section 3.4.1.1 (ATT_ERROR_RSP).
type AttributeProtocolError struct {
	// Code is the 1-byte ATT error code.
	Code uint8
	// Name is a short identifier for the error.
	Name string
	// Description is the human-readable description from the spec.
	Description string
}

func (e *AttributeProtocolError) Error() string {
	return fmt.Sprintf("ATT error 0x%02X (%s): %s", e.Code, e.Name, e.Description)
}

// ATT error codes from the Bluetooth Core Specification, Table 3.4.
var (
	ErrAttInvalidHandle              = &AttributeProtocolError{0x01, "Invalid Handle", "The attribute handle given was not valid on this server."}
	ErrAttReadNotPermitted           = &AttributeProtocolError{0x02, "Read Not Permitted", "The attribute cannot be read."}
	ErrAttWriteNotPermitted          = &AttributeProtocolError{0x03, "Write Not Permitted", "The attribute cannot be written."}
	ErrAttInvalidPDU                 = &AttributeProtocolError{0x04, "Invalid PDU", "The attribute PDU was invalid."}
	ErrAttInsufficientAuthentication = &AttributeProtocolError{0x05, "Insufficient Authentication", "The attribute requires authentication before it can be read or written."}
	ErrAttRequestNotSupported        = &AttributeProtocolError{0x06, "Request Not Supported", "ATT Server does not support the request received from the client."}
	ErrAttInvalidOffset              = &AttributeProtocolError{0x07, "Invalid Offset", "Offset specified was past the end of the attribute."}
	ErrAttInsufficientAuthorization  = &AttributeProtocolError{0x08, "Insufficient Authorization", "The attribute requires authorization before it can be read or written."}
	ErrAttPrepareQueueFull           = &AttributeProtocolError{0x09, "Prepare Queue Full", "Too many prepare writes have been queued."}
	ErrAttNotFound                   = &AttributeProtocolError{0x0A, "Attribute Not Found", "No attribute found within the given attribute handle range."}
	ErrAttNotLong                    = &AttributeProtocolError{0x0B, "Attribute Not Long", "The attribute cannot be read using the ATT_READ_BLOB_REQ PDU."}
	ErrAttInsufficientEncKeySize     = &AttributeProtocolError{0x0C, "Encryption Key Size Too Short", "The Encryption Key Size used for encrypting this link is too short."}
	ErrAttInvalidLength              = &AttributeProtocolError{0x0D, "Invalid Attribute Value Length", "The attribute value length is invalid for the operation."}
	ErrAttUnlikelyError              = &AttributeProtocolError{0x0E, "Unlikely Error", "The attribute request that was requested has encountered an error that was unlikely, and therefore could not be completed as requested."}
	ErrAttInsufficientEncryption     = &AttributeProtocolError{0x0F, "Insufficient Encryption", "The attribute requires encryption before it can be read or written."}
	ErrAttUnsupportedGroupType       = &AttributeProtocolError{0x10, "Unsupported Group Type", "The attribute type is not a supported grouping attribute as defined by a higher layer specification."}
	ErrAttInsufficientResources      = &AttributeProtocolError{0x11, "Insufficient Resources", "Insufficient Resources to complete the request."}
	ErrAttOutOfSync                  = &AttributeProtocolError{0x12, "Database Out Of Sync", "The server requests the client to rediscover the database."}
	ErrAttValueNotAllowed            = &AttributeProtocolError{0x13, "Value Not Allowed", "The attribute parameter value was not allowed."}
)

// attErrorsByCode maps ATT error codes to their predefined error values.
var attErrorsByCode = map[uint8]*AttributeProtocolError{
	0x01: ErrAttInvalidHandle,
	0x02: ErrAttReadNotPermitted,
	0x03: ErrAttWriteNotPermitted,
	0x04: ErrAttInvalidPDU,
	0x05: ErrAttInsufficientAuthentication,
	0x06: ErrAttRequestNotSupported,
	0x07: ErrAttInvalidOffset,
	0x08: ErrAttInsufficientAuthorization,
	0x09: ErrAttPrepareQueueFull,
	0x0A: ErrAttNotFound,
	0x0B: ErrAttNotLong,
	0x0C: ErrAttInsufficientEncKeySize,
	0x0D: ErrAttInvalidLength,
	0x0E: ErrAttUnlikelyError,
	0x0F: ErrAttInsufficientEncryption,
	0x10: ErrAttUnsupportedGroupType,
	0x11: ErrAttInsufficientResources,
	0x12: ErrAttOutOfSync,
	0x13: ErrAttValueNotAllowed,
}

// ATTErrorFromCode returns an *AttributeProtocolError for the given ATT error code.
// For known codes it returns the predefined variable; for application errors (0x80–0x9F)
// and common profile/service errors (0xE0–0xFF) it returns a dynamically created error.
// Unknown codes return a generic ATT error.
func ATTErrorFromCode(code uint8) *AttributeProtocolError {
	if err, ok := attErrorsByCode[code]; ok {
		return err
	}
	if code >= 0x80 && code <= 0x9F {
		return &AttributeProtocolError{code, "Application Error", fmt.Sprintf("Application error code defined by a higher layer specification (0x%02X).", code)}
	}
	if code >= 0xE0 && code <= 0xFF {
		return &AttributeProtocolError{code, "Common Profile and Service Error", fmt.Sprintf("Common profile and service error code (0x%02X).", code)}
	}
	return &AttributeProtocolError{code, "Reserved", fmt.Sprintf("Reserved for future use (0x%02X).", code)}
}
