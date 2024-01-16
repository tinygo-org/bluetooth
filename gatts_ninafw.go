//go:build ninafw

package bluetooth

type Characteristic struct {
	adapter     *Adapter
	handle      uint16
	permissions CharacteristicPermissions
	value       []byte
	cccd        uint16
}

// AddService creates a new service with the characteristics listed in the
// Service struct.
func (a *Adapter) AddService(service *Service) error {
	uuid := service.UUID.Bytes()
	serviceHandle := a.att.addLocalAttribute(attributeTypeService, 0, shortUUID(gattServiceUUID).UUID(), 0, uuid[:])
	valueHandle := serviceHandle
	endHandle := serviceHandle

	for i := range service.Characteristics {
		data := service.Characteristics[i].UUID.Bytes()
		cuuid := append([]byte{}, data[:]...)

		// add characteristic declaration
		charHandle := a.att.addLocalAttribute(attributeTypeCharacteristic, serviceHandle, shortUUID(gattCharacteristicUUID).UUID(), CharacteristicReadPermission, cuuid[:])

		// add characteristic value
		vf := CharacteristicPermissions(0)
		if service.Characteristics[i].Flags.Read() {
			vf |= CharacteristicReadPermission
		}
		if service.Characteristics[i].Flags.Write() {
			vf |= CharacteristicWritePermission
		}
		valueHandle = a.att.addLocalAttribute(attributeTypeCharacteristicValue, charHandle, service.Characteristics[i].UUID, vf, service.Characteristics[i].Value)
		endHandle = valueHandle

		// add characteristic descriptor
		if service.Characteristics[i].Flags.Notify() ||
			service.Characteristics[i].Flags.Indicate() {
			endHandle = a.att.addLocalAttribute(attributeTypeDescriptor, charHandle, shortUUID(gattClientCharacteristicConfigUUID).UUID(), CharacteristicReadPermission|CharacteristicWritePermission, []byte{0, 0})
		}

		if service.Characteristics[i].Handle != nil {
			service.Characteristics[i].Handle.adapter = a
			service.Characteristics[i].Handle.handle = valueHandle
			service.Characteristics[i].Handle.permissions = service.Characteristics[i].Flags
			service.Characteristics[i].Handle.value = service.Characteristics[i].Value
		}

		if debug {
			println("added characteristic", charHandle, valueHandle, service.Characteristics[i].UUID.String())
		}

		a.att.addLocalCharacteristic(charHandle, service.Characteristics[i].Flags, valueHandle, service.Characteristics[i].UUID, service.Characteristics[i].Handle)
	}

	if debug {
		println("added service", serviceHandle, endHandle, service.UUID.String())
	}

	a.att.addLocalService(serviceHandle, endHandle, service.UUID)

	return nil
}

// Write replaces the characteristic value with a new value.
func (c *Characteristic) Write(p []byte) (n int, err error) {
	if !c.permissions.Notify() {
		return 0, errNoNotify
	}

	c.value = append([]byte{}, p...)

	if c.cccd&0x01 != 0 {
		// send notification
		c.adapter.att.sendNotification(c.handle, c.value)
	}

	return len(c.value), nil
}

func (c *Characteristic) readCCCD() (uint16, error) {
	if !c.permissions.Notify() {
		return 0, errNoNotify
	}

	return c.cccd, nil
}

func (c *Characteristic) writeCCCD(val uint16) error {
	if !c.permissions.Notify() {
		return errNoNotify
	}

	c.cccd = val

	return nil
}

func (c *Characteristic) readValue() ([]byte, error) {
	if !c.permissions.Read() {
		return nil, errNoRead
	}

	return c.value, nil
}
