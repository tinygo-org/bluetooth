package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go Bluetooth",
		ManufacturerData: []bluetooth.ManufacturerDataElement{
			{CompanyID: 0xffff, Data: []byte{0x01, 0x02}},
		},
	}))
	must("start adv", adv.Start())

	println("advertising...")
	address, _ := adapter.Address()
	for {
		println("Go Bluetooth /", address.MAC.String())
		time.Sleep(time.Second)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
