// +build !baremetal

package bluetooth

import (
	"github.com/muka/go-bluetooth/api/service"
)

// AddService creates a new service with the characteristics listed in the
// Service struct.
//
// TODO: add support for characteristics on Linux.
func (a *Adapter) AddService(s *Service) error {
	app, err := service.NewApp(a.id)
	if err != nil {
		return err
	}

	bluezService, err := app.NewService()
	if err != nil {
		return err
	}
	bluezService.Properties.UUID = s.UUID.String()

	// TODO: add support for characteristics

	err = app.AddService(bluezService)
	if err != nil {
		return err
	}

	return app.Run()
}
