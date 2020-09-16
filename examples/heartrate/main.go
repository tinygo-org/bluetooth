package main

import (
	"math/rand"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

// TODO: use atomics to access this value.
var heartRate uint8 = 75 // 75bpm

func main() {
	println("starting")
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "Go HRS",
		ServiceUUIDs: []bluetooth.UUID{bluetooth.New16BitUUID(0x2A37)},
	}))
	must("start adv", adv.Start())

	var heartRateMeasurement bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.New16BitUUID(0x180D), // Heart Rate
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &heartRateMeasurement,
				UUID:   bluetooth.New16BitUUID(0x2A37), // Heart Rate Measurement
				Value:  []byte{0, heartRate},
				Flags: bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission |
					bluetooth.CharacteristicNotifyPermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if offset != 0 || len(value) < 2 {
						return
					}
					if value[1] != 0 { // avoid divide by zero
						heartRate = value[1]
						println("heart rate is now:", heartRate)
					}
				},
			},
		},
	}))

	nextBeat := time.Now()
	for {
		nextBeat = nextBeat.Add(time.Minute / time.Duration(heartRate))
		println("tick", time.Now().Format("04:05.000"))
		time.Sleep(nextBeat.Sub(time.Now()))

		// random variation in heartrate
		heartRate = randomInt(65, 85)

		// and push the next notification
		heartRateMeasurement.Write([]byte{byte(heartRate)})
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
