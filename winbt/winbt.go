// Package winbt provides a thin layer over the WinRT Bluetooth interfaces. It
// is not designed to be used directly by applications: the bluetooth package
// will wrap the API exposed here in a nice platform-independent way.
//
// You can find the original *.idl and *.h files in a directory like this,
// after installing the Windows SDK:
//
//     C:\Program Files (x86)\Windows Kits\10\Include\10.0.19041.0\winrt
//
// Some helpful articles to understand WinRT at a low level:
// https://blog.xojo.com/2019/07/02/accessing-windows-runtime-winrt/
// https://docs.microsoft.com/en-us/archive/msdn-magazine/2013/august/windows-with-c-the-windows-runtime-application-model
// https://blog.magnusmontin.net/2017/12/30/minimal-uwp-wrl-xaml-app/
// https://yizhang82.dev/what-is-winrt
// https://www.slideshare.net/goldshtn/deep-dive-into-winrt
//
package winbt // import "tinygo.org/x/bluetooth/winbt"

import (
	"github.com/go-ole/go-ole"
)

var (
	IID_IBluetoothLEAdvertisementReceivedEventArgs       = ole.NewGUID("27987DDF-E596-41BE-8D43-9E6731D4A913")
	IID_IBluetoothLEAdvertisementWatcherStoppedEventArgs = ole.NewGUID("DD40F84D-E7B9-43E3-9C04-0685D085FD8C")
)

// printGUIDs prints the GUIDs this IInspectable implements. It is primarily
// intended for debugging.
func printGUIDs(inspectable *ole.IInspectable) {
	guids, err := inspectable.GetIids()
	if err != nil {
		println("could not get GUIDs for IInspectable:", err.Error())
		return
	}
	for _, guid := range guids {
		println("guid:", guid.String())
	}
}

// makeError makes a *ole.OleError if hr is non-nil. If it is nil, it will
// return nil.
// This is an utility function to easily convert an HRESULT into a Go error
// value.
func makeError(hr uintptr) error {
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

// mustSucceed can be called to check the return value of getters, which should
// always succeed. If hr is non-zero, it will panic with an error message.
func mustSucceed(hr uintptr) {
	if hr != 0 {
		// Status is a getter, so should never return an error unless
		// an invalid `v` is passed in (for example, `v` is nil) - in
		// which case, there is definitely a bug and we should fail
		// early.
		panic("winbt: unexpected error: " + ole.NewError(hr).String())
	}
}
