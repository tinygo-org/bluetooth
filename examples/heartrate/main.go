package main

import (
	"time"

	"github.com/tinygo-org/bluetooth"
)

// flags + local name
var advPayload = []byte("\x02\x01\x06" + "\x07\x09TinyGo")

var adapter = bluetooth.DefaultAdapter

// TODO: use atomics to access this value.
var heartRate uint8 = 75 // 75bpm

func main() {
	println("starting")
	adapter.SetEventHandler(handleBluetoothEvents)
	must("enable BLE stack", adapter.Enable())
	adv := adapter.NewAdvertisement()
	options := &bluetooth.AdvertiseOptions{
		Interval: bluetooth.NewAdvertiseInterval(100),
	}
	must("config adv", adv.Configure(advPayload, nil, options))
	must("start adv", adv.Start())

	var heartRateMeasurement bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.New16BitUUID(0x180D), // Heart Rate
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &heartRateMeasurement,
				UUID:   bluetooth.New16BitUUID(0x2A37), // Heart Rate Measurement
				Value:  []byte{0, heartRate},
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission,
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
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}

// handleBluetoothEvents prints BLE events as they happen.
func handleBluetoothEvents(evt bluetooth.Event) {
	switch evt := evt.(type) {
	case *bluetooth.ConnectEvent:
		println("evt: connected", evt.Connection)
	case *bluetooth.DisconnectEvent:
		println("evt: disconnected", evt.Connection)
	default:
		println("evt: unknown")
	}
}
