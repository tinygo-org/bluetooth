//go:build (softdevice && s113v7) || (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

/*
#include "nrf_soc.h"
*/
import "C"

import (
	"errors"
	"machine"
	"runtime"
	"sync"
	"unsafe"
)

var (
	errFlashPastEOF   = errors.New("bluetooth: flash access past end of flash data area")
	errFlashUnaligned = errors.New("bluetooth: flash write offset must be word-aligned")
)

// flashOpMu serializes SoftDevice flash operations (bond persistence and
// SDFlash), since their completion is reported through the single
// flashOpResult flag.
var flashOpMu sync.Mutex

// SDFlash is a drop-in replacement for machine.Flash (machine.BlockDevice,
// same offsets and sizes) that performs erase and write operations through
// the SoftDevice (sd_flash_page_erase / sd_flash_write).
//
// While the SoftDevice is enabled, direct NVMC access is forbidden. Note
// that machine.Flash's own SoftDevice-aware path also does not work together
// with this package: it polls sd_evt_get for the completion SOC event, but
// the SWI2 interrupt handler in adapter_sd.go drains those events first, so
// machine.Flash waits forever. Use this type instead after Adapter.Enable.
// Its methods must be called from goroutine context, not from an interrupt.
type SDFlash struct{}

var _ machine.BlockDevice = SDFlash{}

const sdFlashPageSize = 4096 // nrf52 flash page size

func (SDFlash) ReadAt(p []byte, off int64) (n int, err error) {
	if machine.FlashDataStart()+uintptr(off)+uintptr(len(p)) > machine.FlashDataEnd() {
		return 0, errFlashPastEOF
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(machine.FlashDataStart()+uintptr(off))), len(p))
	copy(p, data)
	return len(p), nil
}

func (SDFlash) WriteAt(p []byte, off int64) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off%4 != 0 {
		return 0, errFlashUnaligned
	}
	if machine.FlashDataStart()+uintptr(off)+uintptr(len(p)) > machine.FlashDataEnd() {
		return 0, errFlashPastEOF
	}
	addr := machine.FlashDataStart() + uintptr(off)

	flashOpMu.Lock()
	defer flashOpMu.Unlock()

	written := 0
	for written < len(p) {
		chunk := len(p) - written
		// A single sd_flash_write may not cross a flash page boundary.
		if room := sdFlashPageSize - int(addr%sdFlashPageSize); chunk > room {
			chunk = room
		}
		// The source must be word-aligned RAM and must stay alive until the
		// asynchronous operation completes.
		words := make([]uint32, (chunk+3)/4)
		buf := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(words)*4)
		for i := range buf {
			buf[i] = 0xff
		}
		copy(buf, p[written:written+chunk])

		flashOpResult.Set(0)
		errCode := C.sd_flash_write((*C.uint32_t)(unsafe.Pointer(addr)), (*C.uint32_t)(unsafe.Pointer(&words[0])), C.uint32_t(len(words)))
		if errCode != 0 {
			return written, makeError(errCode)
		}
		ok := waitFlashOp()
		runtime.KeepAlive(words)
		if !ok {
			return written, errFlashOpFailed
		}
		written += chunk
		addr += uintptr(chunk)
	}
	return len(p), nil
}

func (SDFlash) Size() int64 {
	return int64(machine.FlashDataEnd() - machine.FlashDataStart())
}

func (SDFlash) WriteBlockSize() int64 {
	return 4
}

func (SDFlash) EraseBlockSize() int64 {
	return sdFlashPageSize
}

func (SDFlash) EraseBlocks(start, length int64) error {
	firstPage := uint32(machine.FlashDataStart()) / sdFlashPageSize

	flashOpMu.Lock()
	defer flashOpMu.Unlock()

	for i := int64(0); i < length; i++ {
		flashOpResult.Set(0)
		errCode := C.sd_flash_page_erase(C.uint32_t(firstPage + uint32(start+i)))
		if errCode != 0 {
			return makeError(errCode)
		}
		if !waitFlashOp() {
			return errFlashOpFailed
		}
	}
	return nil
}
