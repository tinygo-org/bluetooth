package bluetooth

import (
	"fmt"
	"syscall"

	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
)

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will return a complete list of
// services.
func (d *Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	// IAsyncOperation<GattDeviceServicesResult>
	getServicesOperation, err := d.device.GetGattServicesWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
	if err != nil {
		return nil, err
	}

	if err := awaitAsyncOperation(getServicesOperation, genericattributeprofile.SignatureGattDeviceServicesResult); err != nil {
		return nil, err
	}

	res, err := getServicesOperation.GetResults()
	if err != nil {
		return nil, err
	}

	servicesResult := (*genericattributeprofile.GattDeviceServicesResult)(res)

	status, err := servicesResult.GetStatus()
	if err != nil {
		return nil, err
	} else if status != genericattributeprofile.GattCommunicationStatusSuccess {
		return nil, fmt.Errorf("could not retrieve device services, operation failed with code %d", status)
	}

	// IVectorView<GattDeviceService>
	servicesVector, err := servicesResult.GetServices()
	if err != nil {
		return nil, err
	}

	// Convert services vector to array
	servicesSize, err := servicesVector.GetSize()
	if err != nil {
		return nil, err
	}

	var services []DeviceService
	for i := uint32(0); i < servicesSize; i++ {
		s, err := servicesVector.GetAt(i)
		if err != nil {
			return nil, err
		}

		srv := (*genericattributeprofile.GattDeviceService)(s)
		guid, err := srv.GetUuid()
		if err != nil {
			return nil, err
		}

		serviceUuid := winRTUuidToUuid(guid)

		// add if in our original list
		addToList := len(uuids) == 0

		for _, uuid := range uuids {
			if serviceUuid.String() == uuid.String() {
				// one of the services we're looking for.
				addToList = true
				break
			}
		}

		if addToList {
			services = append(services, DeviceService{
				uuidWrapper: serviceUuid,
				service:     srv,
			})
		}
	}

	return services, nil
}

func winRTUuidToUuid(uuid syscall.GUID) UUID {
	return NewUUID([16]byte{
		byte(uuid.Data1 >> 24),
		byte(uuid.Data1 >> 16),
		byte(uuid.Data1 >> 8),
		byte(uuid.Data1),
		byte(uuid.Data2 >> 8),
		byte(uuid.Data2),
		byte(uuid.Data3 >> 8),
		byte(uuid.Data3),
		uuid.Data4[0], uuid.Data4[1],
		uuid.Data4[2], uuid.Data4[3],
		uuid.Data4[4], uuid.Data4[5],
		uuid.Data4[6], uuid.Data4[7],
	})
}

// uuidWrapper is a type alias for UUID so we ensure no conflicts with
// struct method of the same name.
type uuidWrapper = UUID

// DeviceService is a BLE service on a connected peripheral device.
type DeviceService struct {
	uuidWrapper

	service *genericattributeprofile.GattDeviceService
}

// UUID returns the UUID for this DeviceService.
func (s *DeviceService) UUID() UUID {
	return s.uuidWrapper
}
