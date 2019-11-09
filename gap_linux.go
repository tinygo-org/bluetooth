// +build !baremetal

package bluetooth

import (
	"errors"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
)

var (
	ErrMalformedAdvertisement = errors.New("bluetooth: malformed advertisement packet")
)

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	adapter       *Adapter
	advertisement *api.Advertisement
	properties    *advertising.LEAdvertisement1Properties
}

// NewAdvertisement creates a new advertisement instance but does not configure
// it.
func (a *Adapter) NewAdvertisement() *Advertisement {
	return &Advertisement{
		adapter: a,
	}
}

// Configure this advertisement.
func (a *Advertisement) Configure(broadcastData, scanResponseData []byte, options *AdvertiseOptions) error {
	if a.advertisement != nil {
		panic("todo: configure advertisement a second time")
	}
	if scanResponseData != nil {
		panic("todo: scan response data")
	}

	// Quick-and-dirty advertisement packet parser.
	a.properties = &advertising.LEAdvertisement1Properties{
		Type:    advertising.AdvertisementTypeBroadcast,
		Timeout: 1<<16 - 1,
	}
	for len(broadcastData) != 0 {
		if len(broadcastData) < 2 {
			return ErrMalformedAdvertisement
		}
		fieldLength := broadcastData[0]
		fieldType := broadcastData[1]
		fieldValue := broadcastData[2 : fieldLength+1]
		if int(fieldLength) > len(broadcastData) {
			return ErrMalformedAdvertisement
		}
		switch fieldType {
		case 1:
			// BLE advertisement flags. Ignore.
		case 9:
			// Complete Local Name.
			a.properties.LocalName = string(fieldValue)
		default:
			return ErrMalformedAdvertisement
		}
		broadcastData = broadcastData[fieldLength+1:]
	}

	return nil
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	if a.advertisement != nil {
		panic("todo: start advertisement a second time")
	}
	_, err := api.ExposeAdvertisement(a.adapter.id, a.properties, uint32(a.properties.Timeout))
	if err != nil {
		return err
	}
	return nil
}
