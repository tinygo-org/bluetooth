//go:build (softdevice && s113v7) || (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

/*
#include "nrf_sdm.h"
#include "nrf_soc.h"
*/
import "C"

import (
	"errors"
	"machine"
	"sync"
	"unsafe"
)

const flashPageSize = 4096

var (
	errFlashPastEnd    = errors.New("bluetooth: flash access past end of flash data area")
	errFlashUnaligned  = errors.New("bluetooth: flash write offset must be word aligned")
	errFlashNotEnabled = errors.New("bluetooth: flash needs an enabled adapter")
)

// flashOpMu serializes flash operations, because they all use flashOpResult.
var flashOpMu sync.Mutex

// The SoftDevice reads the source until the write completes, so it must stay
// in RAM. flashOpMu protects it.
var flashWriteBuf [64]uint32

var softdeviceEnabled C.uint8_t

// FlashBlockDevice is a machine.BlockDevice that erases and writes the flash
// data area through the SoftDevice. Get it with Adapter.Flash.
type FlashBlockDevice struct{}

var _ machine.BlockDevice = FlashBlockDevice{}

// Flash returns the flash data area without the bond storage page, after Enable.
// Use it in place of machine.Flash, which cannot write while the SoftDevice runs.
// See https://infocenter.nordicsemi.com/topic/sds_s132/SDS/s1xx/sd_resource_reqs/hw_block_interrupt_vector.html
func (a *Adapter) Flash() (FlashBlockDevice, error) {
	if errCode := C.sd_softdevice_is_enabled(&softdeviceEnabled); errCode != 0 {
		return FlashBlockDevice{}, makeError(errCode)
	}
	if softdeviceEnabled == 0 {
		return FlashBlockDevice{}, errFlashNotEnabled
	}
	return FlashBlockDevice{}, nil
}

func flashStart() uintptr {
	return machine.FlashDataStart()
}

func flashEnd() uintptr {
	return bondFlashAddr()
}

func (FlashBlockDevice) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || flashStart()+uintptr(off)+uintptr(len(p)) > flashEnd() {
		return 0, errFlashPastEnd
	}
	copy(p, unsafe.Slice((*byte)(unsafe.Pointer(flashStart()+uintptr(off))), len(p)))
	return len(p), nil
}

func (FlashBlockDevice) WriteAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off%4 != 0 {
		return 0, errFlashUnaligned
	}
	if off < 0 || flashStart()+uintptr(off)+uintptr(len(p)) > flashEnd() {
		return 0, errFlashPastEnd
	}
	addr := flashStart() + uintptr(off)

	flashOpMu.Lock()
	defer flashOpMu.Unlock()

	buf := unsafe.Slice((*byte)(unsafe.Pointer(&flashWriteBuf[0])), len(flashWriteBuf)*4)
	written := 0
	for written < len(p) {
		// One write must not cross a page boundary.
		chunk := min(len(p)-written, len(buf), flashPageSize-int(addr%flashPageSize))
		words := (chunk + 3) / 4
		copy(buf, p[written:written+chunk])
		for i := chunk; i < words*4; i++ {
			buf[i] = 0xff
		}
		flashOpResult.Set(0)
		errCode := C.sd_flash_write((*C.uint32_t)(unsafe.Pointer(addr)), (*C.uint32_t)(unsafe.Pointer(&flashWriteBuf[0])), C.uint32_t(words))
		if err := finishFlashOp(errCode); err != nil {
			return written, err
		}
		written += chunk
		addr += uintptr(chunk)
	}
	return written, nil
}

func (FlashBlockDevice) Size() int64 {
	return int64(flashEnd() - flashStart())
}

func (FlashBlockDevice) WriteBlockSize() int64 {
	return 4
}

func (FlashBlockDevice) EraseBlockSize() int64 {
	return flashPageSize
}

func (d FlashBlockDevice) EraseBlocks(start, length int64) error {
	if start < 0 || length < 0 || (start+length)*flashPageSize > d.Size() {
		return errFlashPastEnd
	}
	firstPage := uint32(flashStart()) / flashPageSize

	flashOpMu.Lock()
	defer flashOpMu.Unlock()

	for i := int64(0); i < length; i++ {
		flashOpResult.Set(0)
		errCode := C.sd_flash_page_erase(C.uint32_t(firstPage + uint32(start+i)))
		if err := finishFlashOp(errCode); err != nil {
			return err
		}
	}
	return nil
}

// finishFlashOp waits for the flash operation that errCode is the result of.
// The caller must hold flashOpMu and clear flashOpResult before the call.
func finishFlashOp(errCode C.uint32_t) error {
	if errCode != 0 {
		return makeError(errCode)
	}
	if !waitFlashOp() {
		return errFlashOpFailed
	}
	return nil
}
