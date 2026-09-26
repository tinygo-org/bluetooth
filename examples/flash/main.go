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

const magic = 0x600df1a5

var adapter = bluetooth.DefaultAdapter

func main() {
	time.Sleep(2 * time.Second)

	must("enable BLE stack", adapter.Enable())

	dev, err := adapter.Flash()
	must("get flash", err)

	// The first page of the flash data area holds the magic value and the count.
	// Flashing a program does not erase this page, so it can hold old data.
	var buf [8]byte
	_, err = dev.ReadAt(buf[:], 0)
	must("read flash", err)
	count := uint32(0)
	if binary.LittleEndian.Uint32(buf[0:4]) == magic {
		count = binary.LittleEndian.Uint32(buf[4:8])
	}
	count++
	println("reset count:", count)

	must("erase flash", dev.EraseBlocks(0, 1))
	binary.LittleEndian.PutUint32(buf[0:4], magic)
	binary.LittleEndian.PutUint32(buf[4:8], count)
	_, err = dev.WriteAt(buf[:], 0)
	must("write flash", err)

	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go Flash",
		ManufacturerData: []bluetooth.ManufacturerDataElement{
			{CompanyID: 0xffff, Data: buf[4:8]},
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
