package bluetooth

import "errors"

var ErrCannotSendWriteWithoutResponse = errors.New("cannot send write without response")
