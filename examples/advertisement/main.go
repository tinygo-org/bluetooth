package main

import (
	"context"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	must("enable BLE stack", adapter.Enable())

	ctx, cancel := context.WithCancel(context.Background())
	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		if connected {
			println("device connected:", device.Address.String())
			return
		}

		println("device disconnected:", device.Address.String())
		cancel()
	})

	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go Bluetooth",
		ManufacturerData: []bluetooth.ManufacturerDataElement{
			{CompanyID: 0xffff, Data: []byte{0x01, 0x02}},
		},
	}))
	must("start adv", adv.Start())
	defer adv.Stop()

	println("advertising...")
	address, _ := adapter.Address()
	for {
		select {
		case <-time.After(1 * time.Second):
			println("Go Bluetooth /", address.MAC.String())
		case <-ctx.Done():
			return
		}
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
