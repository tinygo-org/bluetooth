package bluetooth

import (
	"bytes"
	"testing"
)

func TestParseMAC(t *testing.T) {
	tests := []struct {
		name    string
		strMac  string
		wantMac MAC
		wantErr bool
	}{
		{
			name:    "incrementing",
			strMac:  "12:34:56:78:9A:BC",
			wantMac: [6]byte{188, 154, 120, 86, 52, 18},
			wantErr: false,
		},
		{
			name:    "decrementing",
			strMac:  "CB:A9:87:65:43:21",
			wantMac: [6]byte{33, 67, 101, 135, 169, 203},
			wantErr: false,
		},
		{
			name:    "normal",
			strMac:  "11:22:33:AA:BB:CC",
			wantMac: [6]byte{204, 187, 170, 51, 34, 17},
			wantErr: false,
		},
		{
			name:    "lower",
			strMac:  "11:22:33:aa:bb:cc",
			wantMac: [6]byte{},
			wantErr: true,
		},
		{
			name:    "longer",
			strMac:  "11:22:33:AA:BB:CC:22",
			wantMac: [6]byte{},
			wantErr: true,
		},
		{
			name:    "extra2",
			strMac:  "11:222:33:AA:BB:CC",
			wantMac: [6]byte{},
			wantErr: true,
		},
		{
			name:    "empty",
			strMac:  "",
			wantMac: [6]byte{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMac, err := ParseMAC(tt.strMac)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMAC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !bytes.Equal(gotMac[:], tt.wantMac[:]) {
				t.Errorf("ParseMAC() = %v, want %v", [6]byte(gotMac), [6]byte(tt.wantMac))
			}
		})
	}
}

func TestMAC_String(t *testing.T) {
	tests := []struct {
		name string
		mac  MAC
		want string
	}{
		{
			name: "incrementing",
			want: "12:34:56:78:9A:BC",
			mac:  [6]byte{188, 154, 120, 86, 52, 18},
		},
		{
			name: "decrementing",
			want: "CB:A9:87:65:43:21",
			mac:  [6]byte{33, 67, 101, 135, 169, 203},
		},
		{
			name: "normal",
			want: "11:22:33:AA:BB:CC",
			mac:  [6]byte{204, 187, 170, 51, 34, 17},
		},
		{
			name: "nil",
			want: "00:00:00:00:00:00",
			mac:  [6]byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mac.String(); got != tt.want {
				t.Errorf("MAC.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkParseMAC(b *testing.B) {
	mac := "CB:A9:87:65:43:21"
	var err error
	for i := 0; i < b.N; i++ {
		_, err = ParseMAC(mac)
		if err != nil {
			b.Errorf("expected nil but got %v", err)
		}
	}
}

func BenchmarkMacToString(b *testing.B) {
	mac, err := ParseMAC("CB:A9:87:65:43:21")
	if err != nil {
		b.Errorf("expected nil but got %v", err)
	}
	for i := 0; i < b.N; i++ {
		_ = mac.String()
	}
}

func TestMAC_UnmarshalBinary(t *testing.T) {
	tests := []struct {
		name    string
		mac     []byte
		wantMac MAC
		wantErr bool
	}{
		{
			name:    "incrementing",
			mac:     []byte{188, 154, 120, 86, 52, 18},
			wantMac: [6]byte{188, 154, 120, 86, 52, 18},
			wantErr: false,
		},
		{
			name:    "decrementing",
			mac:     []byte{33, 67, 101, 135, 169, 203},
			wantMac: [6]byte{33, 67, 101, 135, 169, 203},
			wantErr: false,
		},
		{
			name:    "normal",
			mac:     []byte{204, 187, 170, 51, 34, 17},
			wantMac: [6]byte{204, 187, 170, 51, 34, 17},
			wantErr: false,
		},
		{
			name:    "empty",
			mac:     []byte{},
			wantMac: [6]byte{},
			wantErr: true,
		},
		{
			name:    "extra",
			mac:     []byte{33, 67, 101, 135, 169, 203, 255},
			wantMac: [6]byte{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMac := MAC{}
			gotMac.UnmarshalBinary(tt.mac)

			if !tt.wantErr && !bytes.Equal(gotMac[:], tt.wantMac[:]) {
				t.Errorf("ParseMAC() = %v, want %v", [6]byte(gotMac), [6]byte(tt.wantMac))
			}
		})
	}
}

func TestMAC_MarshalBinary(t *testing.T) {
	tests := []struct {
		name     string
		mac      MAC
		wantData []byte
		wantErr  bool
	}{
		{
			name:     "incrementing",
			mac:      [6]byte{188, 154, 120, 86, 52, 18},
			wantData: []byte{188, 154, 120, 86, 52, 18},
			wantErr:  false,
		},
		{
			name:     "decrementing",
			mac:      [6]byte{33, 67, 101, 135, 169, 203},
			wantData: []byte{33, 67, 101, 135, 169, 203},
			wantErr:  false,
		},
		{
			name:     "normal",
			mac:      [6]byte{204, 187, 170, 51, 34, 17},
			wantData: []byte{204, 187, 170, 51, 34, 17},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotData, err := tt.mac.MarshalBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("MAC.MarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytes.Equal(gotData, tt.wantData) {
				t.Errorf("MAC.MarshalBinary() = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}

func TestMAC_MarshalUnmarshalBinary(t *testing.T) {
	tests := []struct {
		name    string
		mac     MAC
		wantMac MAC
		wantErr bool
	}{
		{
			name:    "incrementing",
			mac:     [6]byte{188, 154, 120, 86, 52, 18},
			wantMac: [6]byte{188, 154, 120, 86, 52, 18},
			wantErr: false,
		},
		{
			name:    "decrementing",
			mac:     [6]byte{33, 67, 101, 135, 169, 203},
			wantMac: [6]byte{33, 67, 101, 135, 169, 203},
			wantErr: false,
		},
		{
			name:    "normal",
			mac:     [6]byte{204, 187, 170, 51, 34, 17},
			wantMac: [6]byte{204, 187, 170, 51, 34, 17},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.mac.MarshalBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("MAC.MarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotMac := &MAC{}

			err = gotMac.UnmarshalBinary(b)
			if (err != nil) != tt.wantErr {
				t.Errorf("MAC.UnmarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !bytes.Equal(gotMac[:], tt.wantMac[:]) {
				t.Errorf("MAC.MarshalBinary() = %v, want %v", gotMac, tt.wantMac)
			}
		})
	}
}
