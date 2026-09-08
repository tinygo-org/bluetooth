// Beacon that uses as little current as possible.
// Build it with -serial=none, so that the USB peripheral stays off.
//
//	tinygo flash -target xiao-ble -serial=none ./examples/beacon-lowpower
package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

// txPower is the radio transmit power in dBm. A lower level uses less current
// but gives less range. Set 0 to keep the default level.
const txPower = 0

var adapter = bluetooth.DefaultAdapter

func main() {
	must("enable BLE stack", adapter.Enable())

	// Remove this if the board has no DC/DC inductor. Not every board gives
	// this control, so a failure is not fatal here.
	adapter.EnableDCSupply(bluetooth.DCSupplyMain, true)

	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		AdvertisementType: bluetooth.AdvertisingTypeNonConnInd,
		Interval:          bluetooth.NewDuration(2 * time.Second),
		ManufacturerData: []bluetooth.ManufacturerDataElement{
			{CompanyID: 0xffff, Data: []byte{0x01, 0x02}},
		},
	}))
	// Not every board gives this control, so a failure here is not fatal.
	adv.SetTxPower(txPower)
	must("start adv", adv.Start())

	// The BLE stack advertises on its own, so park the CPU. Each wake up uses
	// current and does no work here.
	for {
		time.Sleep(time.Hour)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
