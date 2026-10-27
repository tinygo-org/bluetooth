package bluetooth

import (
	"reflect"
	"testing"
)

func TestSetConnectHandler(t *testing.T) {
	a := &Adapter{}
	var got []bool
	a.SetConnectHandler(func(d Device, connected bool) { got = append(got, connected) })

	a.connectHandler(Device{}, ConnectionState{Connected: true})
	a.connectHandler(Device{}, ConnectionState{Err: DisconnectTimeout})

	if want := []bool{true, false}; !reflect.DeepEqual(got, want) {
		t.Errorf("handler got %v, want %v", got, want)
	}

	a.SetConnectHandler(nil)
	a.connectHandler(Device{}, ConnectionState{Connected: true})
}
