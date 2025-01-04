// this example implements a BLE heart rate sensor.
// see https://www.bluetooth.com/specifications/specs/heart-rate-profile-1-0/ for the full spec.
package main

import (
	"math/rand"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter = bluetooth.DefaultAdapter

	heartRateMeasurement bluetooth.Characteristic
	bodyLocation         bluetooth.Characteristic
	controlPoint         bluetooth.Characteristic

	heartRate uint8 = 75 // 75bpm
)

func main() {
	println("starting")
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "Go HRS",
		ServiceUUIDs: []bluetooth.UUID{bluetooth.ServiceUUIDHeartRate},
	}))
	must("start adv", adv.Start())

	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDHeartRate,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &heartRateMeasurement,
				UUID:   bluetooth.CharacteristicUUIDHeartRateMeasurement,
				Value:  []byte{0, heartRate},
				Flags:  bluetooth.CharacteristicNotifyPermission,
			},
			{
				Handle: &bodyLocation,
				UUID:   bluetooth.CharacteristicUUIDBodySensorLocation,
				Value:  []byte{1}, // "Chest"
				Flags:  bluetooth.CharacteristicReadPermission,
			},
			{
				Handle: &controlPoint,
				UUID:   bluetooth.CharacteristicUUIDHeartRateControlPoint,
				Value:  []byte{0},
				Flags:  bluetooth.CharacteristicWritePermission,
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
		heartRateMeasurement.Write([]byte{0, heartRate})
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
