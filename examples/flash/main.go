// Counts the resets of the board in flash, and advertises the count.
// It uses Adapter.Flash, because machine.Flash cannot erase or write while
// the SoftDevice is enabled.
//
//	tinygo flash -target xiao-ble -monitor ./examples/flash
package main

import (
	"encoding/binary"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	time.Sleep(2 * time.Second)

	must("enable BLE stack", adapter.Enable())

	dev, err := adapter.Flash()
	must("get flash", err)

	// Use the first page of the flash data area.
	var buf [4]byte
	_, err = dev.ReadAt(buf[:], 0)
	must("read flash", err)
	count := binary.LittleEndian.Uint32(buf[:])
	if count == 0xffffffff {
		count = 0
	}
	count++
	println("reset count:", count)

	must("erase flash", dev.EraseBlocks(0, 1))
	binary.LittleEndian.PutUint32(buf[:], count)
	_, err = dev.WriteAt(buf[:], 0)
	must("write flash", err)

	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go Flash",
		ManufacturerData: []bluetooth.ManufacturerDataElement{
			{CompanyID: 0xffff, Data: buf[:]},
		},
	}))
	must("start adv", adv.Start())

	for {
		time.Sleep(time.Hour)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
