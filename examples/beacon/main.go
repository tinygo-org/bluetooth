package main

import (
	"encoding/binary"
	"math/rand"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var (
	temperatureData [2]byte
	sd              = []bluetooth.ServiceDataElement{
		{
			UUID: bluetooth.CharacteristicUUIDTemperature,
			Data: temperatureData[:],
		},
	}

	opts = bluetooth.AdvertisementOptions{
		LocalName:         "Go Bluetooth",
		ServiceData:       sd,
		AdvertisementType: bluetooth.AdvertisingTypeScanInd,
	}
)

func main() {
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()

	println("advertising...")
	address, _ := adapter.Address()
	for {
		setServiceData(randomInt(100, 500))
		must("config adv", adv.Configure(opts))
		must("start adv", adv.Start())

		println("Go Bluetooth /", address.MAC.String())
		time.Sleep(time.Second)
		adv.Stop()
	}
}

func setServiceData(m1 uint16) {
	binary.LittleEndian.PutUint16(temperatureData[:], m1)
}

// Returns an int >= min, < max
func randomInt(min, max int) uint16 {
	return uint16(min + rand.Intn(max-min))
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
