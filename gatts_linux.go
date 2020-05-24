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

	// TODO: add support for characteristics

	err = app.AddService(bluezService)
	if err != nil {
		return err
	}

	return app.Run()
}
