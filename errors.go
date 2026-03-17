package bluetooth

import (
	"encoding/hex"
	"strings"
)

// AttributeProtocolError represents an ATT error code as defined in the
// Bluetooth Core Specification, Section 3.4.1.1 (ATT_ERROR_RSP), Table 3.4.
type AttributeProtocolError uint8

// attErrorDetails holds the name and description for an ATT error code.
type attErrorDetails struct {
	// name is a short identifier for the error.
	name string
	// description is the human-readable description from the spec.
	description string
}

// ATT error codes from the Bluetooth Core Specification, Table 3.4.
const (
	ErrAttInvalidHandle              AttributeProtocolError = 0x01 // The attribute handle given was not valid on this server.
	ErrAttReadNotPermitted           AttributeProtocolError = 0x02 // The attribute cannot be read.
	ErrAttWriteNotPermitted          AttributeProtocolError = 0x03 // The attribute cannot be written.
	ErrAttInvalidPDU                 AttributeProtocolError = 0x04 // The attribute PDU was invalid.
	ErrAttInsufficientAuthentication AttributeProtocolError = 0x05 // The attribute requires authentication before it can be read or written.
	ErrAttRequestNotSupported        AttributeProtocolError = 0x06 // ATT Server does not support the request received from the client.
	ErrAttInvalidOffset              AttributeProtocolError = 0x07 // Offset specified was past the end of the attribute.
	ErrAttInsufficientAuthorization  AttributeProtocolError = 0x08 // The attribute requires authorization before it can be read or written.
	ErrAttPrepareQueueFull           AttributeProtocolError = 0x09 // Too many prepare writes have been queued.
	ErrAttNotFound                   AttributeProtocolError = 0x0A // No attribute found within the given attribute handle range.
	ErrAttNotLong                    AttributeProtocolError = 0x0B // The attribute cannot be read using the ATT_READ_BLOB_REQ PDU.
	ErrAttInsufficientEncKeySize     AttributeProtocolError = 0x0C // The Encryption Key Size used for encrypting this link is too short.
	ErrAttInvalidLength              AttributeProtocolError = 0x0D // The attribute value length is invalid for the operation.
	ErrAttUnlikelyError              AttributeProtocolError = 0x0E // The attribute request has encountered an error that was unlikely, and therefore could not be completed as requested.
	ErrAttInsufficientEncryption     AttributeProtocolError = 0x0F // The attribute requires encryption before it can be read or written.
	ErrAttUnsupportedGroupType       AttributeProtocolError = 0x10 // The attribute type is not a supported grouping attribute as defined by a higher layer specification.
	ErrAttInsufficientResources      AttributeProtocolError = 0x11 // Insufficient Resources to complete the request.
	ErrAttOutOfSync                  AttributeProtocolError = 0x12 // The server requests the client to rediscover the database.
	ErrAttValueNotAllowed            AttributeProtocolError = 0x13 // The attribute parameter value was not allowed.
)

var attErrors = map[AttributeProtocolError]attErrorDetails{
	ErrAttInvalidHandle:              {"Invalid Handle", "The attribute handle given was not valid on this server."},
	ErrAttReadNotPermitted:           {"Read Not Permitted", "The attribute cannot be read."},
	ErrAttWriteNotPermitted:          {"Write Not Permitted", "The attribute cannot be written."},
	ErrAttInvalidPDU:                 {"Invalid PDU", "The attribute PDU was invalid."},
	ErrAttInsufficientAuthentication: {"Insufficient Authentication", "The attribute requires authentication before it can be read or written."},
	ErrAttRequestNotSupported:        {"Request Not Supported", "ATT Server does not support the request received from the client."},
	ErrAttInvalidOffset:              {"Invalid Offset", "Offset specified was past the end of the attribute."},
	ErrAttInsufficientAuthorization:  {"Insufficient Authorization", "The attribute requires authorization before it can be read or written."},
	ErrAttPrepareQueueFull:           {"Prepare Queue Full", "Too many prepare writes have been queued."},
	ErrAttNotFound:                   {"Attribute Not Found", "No attribute found within the given attribute handle range."},
	ErrAttNotLong:                    {"Attribute Not Long", "The attribute cannot be read using the ATT_READ_BLOB_REQ PDU."},
	ErrAttInsufficientEncKeySize:     {"Encryption Key Size Too Short", "The Encryption Key Size used for encrypting this link is too short."},
	ErrAttInvalidLength:              {"Invalid Attribute Value Length", "The attribute value length is invalid for the operation."},
	ErrAttUnlikelyError:              {"Unlikely Error", "The attribute request has encountered an error that was unlikely, and therefore could not be completed as requested."},
	ErrAttInsufficientEncryption:     {"Insufficient Encryption", "The attribute requires encryption before it can be read or written."},
	ErrAttUnsupportedGroupType:       {"Unsupported Group Type", "The attribute type is not a supported grouping attribute as defined by a higher layer specification."},
	ErrAttInsufficientResources:      {"Insufficient Resources", "Insufficient Resources to complete the request."},
	ErrAttOutOfSync:                  {"Database Out Of Sync", "The server requests the client to rediscover the database."},
	ErrAttValueNotAllowed:            {"Value Not Allowed", "The attribute parameter value was not allowed."},
}

// Code returns the raw ATT error code.
func (e AttributeProtocolError) Code() uint8 {
	return uint8(e)
}

func (e AttributeProtocolError) details() attErrorDetails {
	if d, ok := attErrors[e]; ok {
		return d
	}
	code := hex.EncodeToString([]byte{uint8(e)})
	if e >= 0x80 && e <= 0x9F {
		return attErrorDetails{"Application Error", "Application error code defined by a higher layer specification (0x" + code + ")."}
	}
	if e >= 0xE0 && e <= 0xFF {
		return attErrorDetails{"Common Profile and Service Error", "Common profile and service error code (0x" + code + ")."}
	}
	return attErrorDetails{"Reserved", "Reserved for future use (0x" + code + ")."}
}

// Name returns a short human-readable name for this ATT error code.
func (e AttributeProtocolError) Name() string {
	return e.details().name
}

// Description returns the full description from the Bluetooth Core Specification.
func (e AttributeProtocolError) Description() string {
	return e.details().description
}

func (e AttributeProtocolError) Error() string {
	d := e.details()
	var b strings.Builder
	b.WriteString("ATT error 0x")
	b.WriteString(hex.EncodeToString([]byte{uint8(e)}))
	b.WriteString(" (")
	b.WriteString(d.name)
	b.WriteString("): ")
	b.WriteString(d.description)
	return b.String()
}
