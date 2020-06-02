// +build !baremetal

package bluetooth

import (
	"github.com/muka/go-bluetooth/api/service"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
)

// AddService creates a new service with the characteristics listed in the
// Service struct.
func (a *Adapter) AddService(s *Service) error {
	app, err := service.NewApp(service.AppOptions{
		AdapterID: a.id,
	})
	if err != nil {
		return err
	}

	bluezService, err := app.NewService(s.UUID.String())
	if err != nil {
		return err
	}

	err = app.AddService(bluezService)
	if err != nil {
		return err
	}

	for _, char := range s.Characteristics {
		// Create characteristic handle.
		bluezChar, err := bluezService.NewChar(char.UUID.String())
		if err != nil {
			return err
		}

		// Set properties.
		bluezCharFlags := []string{
			gatt.FlagCharacteristicBroadcast,            // bit 0
			gatt.FlagCharacteristicRead,                 // bit 1
			gatt.FlagCharacteristicWriteWithoutResponse, // bit 2
			gatt.FlagCharacteristicWrite,                // bit 3
			gatt.FlagCharacteristicNotify,               // bit 4
			gatt.FlagCharacteristicIndicate,             // bit 5
		}
		for i := uint(0); i < 5; i++ {
			if (char.Flags>>i)&1 != 0 {
				bluezChar.Properties.Flags = append(bluezChar.Properties.Flags, bluezCharFlags[i])
			}
		}
		bluezChar.Properties.Value = char.Value

		// Do a callback when the value changes.
		if char.WriteEvent != nil {
			callback := char.WriteEvent
			bluezChar.OnWrite(func(c *service.Char, value []byte) ([]byte, error) {
				// BlueZ doesn't seem to tell who did the write, so pass 0
				// always.
				// It also doesn't provide which part of the value was written,
				// so pretend the entire characteristic was updated (which might
				// not be the case).
				callback(0, 0, value)
				return nil, nil
			})
		}

		// Add characteristic to the service, to activate it.
		err = bluezService.AddChar(bluezChar)
		if err != nil {
			return err
		}
	}

	return app.Run()
}
