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
func (d *Device) DiscoverServices(filterUUIDs []UUID) ([]DeviceService, error) {
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

		// only include services that are included in the input filter
		if len(filterUUIDs) > 0 {
			found := false
			for _, uuid := range filterUUIDs {
				if serviceUuid.String() == uuid.String() {
					// One of the services we're looking for.
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		services = append(services, DeviceService{
			uuidWrapper: serviceUuid,
			service:     srv,
		})
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

// DiscoverCharacteristics discovers characteristics in this service. Pass a
// list of characteristic UUIDs you are interested in to this function. Either a
// list of all requested characteristics is returned, or if some characteristics could not be
// discovered an error is returned. If there is no error, the characteristics
// slice has the same length as the UUID slice with characteristics in the same
// order in the slice as in the requested UUID list.
//
// Passing a nil slice of UUIDs will return a complete
// list of characteristics.
func (s *DeviceService) DiscoverCharacteristics(filterUUIDs []UUID) ([]DeviceCharacteristic, error) {
	getCharacteristicsOp, err := s.service.GetCharacteristicsWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
	if err != nil {
		return nil, err
	}

	// IAsyncOperation<GattCharacteristicsResult>
	if err := awaitAsyncOperation(getCharacteristicsOp, genericattributeprofile.SignatureGattCharacteristicsResult); err != nil {
		return nil, err
	}

	res, err := getCharacteristicsOp.GetResults()
	if err != nil {
		return nil, err
	}

	gattCharResult := (*genericattributeprofile.GattCharacteristicsResult)(res)

	// IVectorView<GattCharacteristic>
	charVector, err := gattCharResult.GetCharacteristics()
	if err != nil {
		return nil, err
	}

	// Convert characteristics vector to array
	characteristicsSize, err := charVector.GetSize()
	if err != nil {
		return nil, err
	}

	var characteristics []DeviceCharacteristic
	for i := uint32(0); i < characteristicsSize; i++ {
		s, err := charVector.GetAt(i)
		if err != nil {
			return nil, err
		}

		characteristic := (*genericattributeprofile.GattCharacteristic)(s)
		guid, err := characteristic.GetUuid()
		if err != nil {
			return nil, err
		}

		characteristicUUID := winRTUuidToUuid(guid)

		properties, err := characteristic.GetCharacteristicProperties()
		if err != nil {
			return nil, err
		}

		// only include characteristics that are included in the input filter
		if len(filterUUIDs) > 0 {
			found := false
			for _, uuid := range filterUUIDs {
				if characteristicUUID.String() == uuid.String() {
					// One of the characteristics we're looking for.
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		characteristics = append(characteristics, DeviceCharacteristic{
			uuidWrapper:    characteristicUUID,
			characteristic: characteristic,
			properties:     properties,
		})
	}

	return characteristics, nil
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device.
type DeviceCharacteristic struct {
	uuidWrapper

	characteristic *genericattributeprofile.GattCharacteristic
	properties     genericattributeprofile.GattCharacteristicProperties
}

// UUID returns the UUID for this DeviceCharacteristic.
func (c *DeviceCharacteristic) UUID() UUID {
	return c.uuidWrapper
}

func (c *DeviceCharacteristic) Properties() uint32 {
	return uint32(c.properties)
}
