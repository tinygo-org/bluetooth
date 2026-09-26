//go:build !(softdevice && (s113v7 || s132v6 || s140v6 || s140v7))

package bluetooth

// FlashBlockDevice is a block device for the flash data area. Only the nRF52
// series with a SoftDevice has it, so it is not supported here.
type FlashBlockDevice struct{}

// Flash returns a block device for the flash data area. It is not supported
// on this platform.
func (a *Adapter) Flash() (FlashBlockDevice, error) {
	return FlashBlockDevice{}, errNotSupported
}

func (FlashBlockDevice) ReadAt(p []byte, off int64) (int, error) {
	return 0, errNotSupported
}

func (FlashBlockDevice) WriteAt(p []byte, off int64) (int, error) {
	return 0, errNotSupported
}

func (FlashBlockDevice) Size() int64 {
	return 0
}

func (FlashBlockDevice) WriteBlockSize() int64 {
	return 0
}

func (FlashBlockDevice) EraseBlockSize() int64 {
	return 0
}

func (FlashBlockDevice) EraseBlocks(start, length int64) error {
	return errNotSupported
}
