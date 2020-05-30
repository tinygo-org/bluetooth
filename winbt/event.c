
// This file implements the C side of WinRT events.
// Unfortunately, this cannot be done entirely in pure go, for two reasons:
//   * An Event object must be shared with the C world (WinRT) after
//     syscall.Syscall returns. This is not allowed in Go, to keep the option
//     open to switch to a moving GC in the future. For this, it is needed to
//     allocate the Event object on the C heap (using malloc).
//   * Building a virtual function table is very difficult (if not impossible)
//     in pure Go. It might be possible using reflect, but due the the previous
//     issue I haven't investigated that.

#include <stdint.h>

// Note: these functions have a different signature but because they are only
// used as function pointers (and never called) and because they use C name
// mangling, the signature doesn't really matter.
void winbt_Event_Invoke(void);
void winbt_Event_QueryInterface(void);

// This is the contract the functions below should adhere to:
// https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nn-unknwn-iunknown

static uint64_t winbt_Event_AddRef(void) {
	// This is safe, see winbt_Event_Release.
	return 2;
}

static uint64_t winbt_Event_Release(void) {
	// Pretend there is one reference left.
	// The docs say:
	// > This value is intended to be used only for test purposes.
	// Also see:
	// https://docs.microsoft.com/en-us/archive/msdn-magazine/2013/august/windows-with-c-the-windows-runtime-application-model
	return 1;
}

// The Vtable structure for WinRT event interfaces.
typedef struct {
	void *QueryInterface;
	void *AddRef;
	void *Release;
	void *Invoke;
} EventVtbl_t;

// The Vtable itself. It can be kept constant.
static const EventVtbl_t winbt_EventVtbl = {
	(void*)winbt_Event_QueryInterface,
	(void*)winbt_Event_AddRef,
	(void*)winbt_Event_Release,
	(void*)winbt_Event_Invoke,
};

// A small helper function to get the Vtable.
const EventVtbl_t * winbt_getEventVtbl(void) {
	return &winbt_EventVtbl;
}
