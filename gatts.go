package bluetooth

// Service is a GATT service to be used in AddService.
type Service struct {
	handle uint16
	UUID
	Characteristics []Characteristic
}

type Characteristic struct {
	handle uint16
	UUID
	Value []byte
}
