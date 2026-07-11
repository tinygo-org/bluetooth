//go:build (softdevice && s113v7) || (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

/*
#include "nrf_soc.h"
*/
import "C"

import (
	"errors"
	"machine"
	"runtime/volatile"
	"time"
	"unsafe"
)

// Bond storage uses the last flash page of the application's flash area (as
// reported by the linker, so it never collides with program code) to persist
// the single bond this backend supports across resets: the LTK plus the
// CCCD state (subscriptions) of the bonded central. A bonded central expects
// both to survive a reset of this device, and does not necessarily subscribe
// to notifications again on reconnection. The page is erased and rewritten
// as a whole every time the bond changes.
const (
	bondFlashPageSize = 4096
	bondFlashMagic    = 0xB0FFEE01 // arbitrary, just not the erased-flash value (0xFFFFFFFF)
)

var errFlashOpFailed = errors.New("bluetooth: flash operation failed")

func bondFlashAddr() uintptr {
	return machine.FlashDataEnd() - bondFlashPageSize
}

// loadBondFromFlash restores a previously persisted bond, if any: the LTK (so
// a bonded central can re-encrypt the link via a
// BLE_GAP_EVT_SEC_INFO_REQUEST) and the CCCD state (so its notification
// subscriptions still apply). It is called once, from EnablePairing, before
// the device has any connection.
func loadBondFromFlash() {
	addr := bondFlashAddr()
	magic := *(*uint32)(unsafe.Pointer(addr))
	if magic != bondFlashMagic {
		return
	}
	encKeyLen := int(unsafe.Sizeof(ownEncKey))
	offset := addr + 4
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&ownEncKey)), encKeyLen), unsafe.Slice((*byte)(unsafe.Pointer(offset)), encKeyLen))
	offset += uintptr(encKeyLen)

	length := *(*uint32)(unsafe.Pointer(offset))
	offset += 4
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&sysAttrBuf[0])), len(sysAttrBuf)), unsafe.Slice((*byte)(unsafe.Pointer(offset)), len(sysAttrBuf)))

	bondValid = true
	if length <= uint32(len(sysAttrBuf)) {
		sysAttrLen.Set(uint16(length))
	}
}

// saveBondToFlash persists the current bond (the LTK in ownEncKey and the
// CCCD state in sysAttrBuf) so it survives a reset. It must only be called
// from the bond storage worker (not interrupt context) and blocks for the
// duration of the flash operation (a few milliseconds).
func saveBondToFlash() error {
	addr := bondFlashAddr()

	flashOpResult.Set(0)
	errCode := C.sd_flash_page_erase(C.uint32_t(uint32(addr) / bondFlashPageSize))
	if errCode != 0 {
		return makeError(errCode)
	}
	if !waitFlashOp() {
		return errFlashOpFailed
	}

	encKeyLen := int(unsafe.Sizeof(ownEncKey))
	recordLen := 4 + encKeyLen + 4 + len(sysAttrBuf)
	buf := make([]byte, (recordLen+3)&^3) // magic, record, padded to a whole number of words
	for i := recordLen; i < len(buf); i++ {
		buf[i] = 0xff
	}
	*(*uint32)(unsafe.Pointer(&buf[0])) = bondFlashMagic
	offset := 4
	copy(buf[offset:offset+encKeyLen], unsafe.Slice((*byte)(unsafe.Pointer(&ownEncKey)), encKeyLen))
	offset += encKeyLen
	*(*uint32)(unsafe.Pointer(&buf[offset])) = uint32(sysAttrLen.Get())
	offset += 4
	copy(buf[offset:offset+len(sysAttrBuf)], unsafe.Slice((*byte)(unsafe.Pointer(&sysAttrBuf[0])), len(sysAttrBuf)))

	flashOpResult.Set(0)
	errCode = C.sd_flash_write((*C.uint32_t)(unsafe.Pointer(addr)), (*C.uint32_t)(unsafe.Pointer(&buf[0])), C.uint32_t(len(buf)/4))
	if errCode != 0 {
		return makeError(errCode)
	}
	if !waitFlashOp() {
		return errFlashOpFailed
	}
	return nil
}

// eraseBondFromFlash erases the persisted bond record. It must only be
// called from the bond storage worker (see bondStorageWorker), so that flash
// operations never run concurrently.
func eraseBondFromFlash() error {
	flashOpResult.Set(0)
	errCode := C.sd_flash_page_erase(C.uint32_t(uint32(bondFlashAddr()) / bondFlashPageSize))
	if errCode != 0 {
		return makeError(errCode)
	}
	if !waitFlashOp() {
		return errFlashOpFailed
	}
	return nil
}

// Erase-request handshake between RemoveBond (application goroutine context)
// and the bond storage worker, in the same volatile-flag style used for all
// other cross-goroutine signalling in this backend. RemoveBond clears
// bondEraseDone, sets bondEraseRequested and then polls bondEraseDone;
// bondEraseErr is only valid once bondEraseDone is set. Only one erase can
// be outstanding at a time (RemoveBond blocks until completion).
var (
	bondEraseRequested volatile.Register8
	bondEraseDone      volatile.Register8
	bondEraseErr       error
)

// bondStorageWorker runs all bond flash operations: deferred saves whenever
// the RAM copy of the bond has changed (bondDirty, set from interrupt
// context) and erase requests from RemoveBond. Routing every flash operation
// through this one goroutine serializes access to the flash and to
// flashOpResult, and keeps the security worker responsive to pairing events
// while a flash operation (up to tens of milliseconds) is in progress.
func bondStorageWorker() {
	for {
		if bondEraseRequested.Get() != 0 {
			bondEraseRequested.Set(0)
			bondEraseErr = eraseBondFromFlash()
			bondEraseDone.Set(1)
			continue
		}
		if bondDirty.Get() != 0 {
			bondDirty.Set(0)
			if err := saveBondToFlash(); debug && err != nil {
				println("failed to persist bond:", err.Error())
			}
			continue
		}
		time.Sleep(16 * time.Millisecond)
	}
}

// waitFlashOp blocks until the SWI2 interrupt handler records the result of
// the flash operation started just before calling this function, and reports
// whether it succeeded. It must be called from thread context (the bond
// storage worker), never from an interrupt. The timeout leaves ample room
// for the slowest flash operation (a page erase takes up to ~90ms, and the
// SoftDevice may delay it to protect radio timing).
func waitFlashOp() bool {
	for i := 0; i < 200; i++ {
		switch flashOpResult.Get() {
		case 1:
			return true
		case 2:
			return false
		}
		time.Sleep(time.Millisecond)
	}
	return false // timed out: the SoftDevice should always raise one of the two events
}
