package bluetooth

import (
	"errors"

	"github.com/tinygo-org/cbgo"
)

// attError returns an ATT error from the peripheral as an AttributeProtocolError.
// CBATTError values in CoreBluetooth's CBError.h are the ATT error codes.
func attError(err error) error {
	var nserr *cbgo.NSError
	if errors.As(err, &nserr) && nserr.Domain() == "CBATTErrorDomain" {
		return AttributeProtocolError(nserr.Code())
	}
	return err
}
