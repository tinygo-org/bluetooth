// example that demonstrates how to create a BLE peripheral device with the Battery Service.
package main

import (
	"math/rand"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var (
	localName = "TinyGo Battery"

	batteryLevel uint8 = 75 // 75%
	battery      bluetooth.Characteristic
)

func main() {
	println("starting")
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    localName,
		ServiceUUIDs: []bluetooth.UUID{bluetooth.ServiceUUIDBattery},
	}))
	must("start adv", adv.Start())

	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDBattery,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &battery,
				UUID:   bluetooth.CharacteristicUUIDBatteryLevel,
				Value:  []byte{byte(batteryLevel)},
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicNotifyPermission,
			},
		},
	}))

	for {
		println("tick", time.Now().Format("04:05.000"))

		// random variation in batteryLevel
		batteryLevel = randomInt(65, 85)

		// and push the next notification
		battery.Write([]byte{batteryLevel})

		time.Sleep(time.Second)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}

// Returns an int >= min, < max
func randomInt(min, max int) uint8 {
	return uint8(min + rand.Intn(max-min))
}
