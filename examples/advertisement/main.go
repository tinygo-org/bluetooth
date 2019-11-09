package main

import (
	"time"

	"github.com/aykevl/go-bluetooth"
)

// flags + local name
var advPayload = []byte("\x02\x01\x06" + "\x07\x09TinyGo")

func main() {
	adapter, err := bluetooth.DefaultAdapter()
	must("get default adapter", err)
	must("enable SD", adapter.Enable())
	adv := adapter.NewAdvertisement()
	options := &bluetooth.AdvertiseOptions{
		Interval: bluetooth.NewAdvertiseInterval(100),
	}
	must("config adv", adv.Configure(advPayload, nil, options))
	must("start adv", adv.Start())

	println("advertising...")
	for {
		// Sleep forever.
		time.Sleep(time.Hour)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
