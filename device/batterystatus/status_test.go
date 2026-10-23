package batterystatus

import "testing"

func TestStatusDecodeAndSonyBands(t *testing.T) {
	for _, tc := range []struct {
		json     string
		ds4, ds5 byte
	}{
		{`{"version":1,"percent":0,"charging":false,"externalPower":false,"complete":false}`, 0, 0},
		{`{"version":1,"percent":50,"charging":false,"externalPower":false,"complete":false}`, 5, 5},
		{`{"version":1,"percent":60,"charging":true,"externalPower":true,"complete":false}`, 0x16, 0x16},
		{`{"version":1,"percent":100,"charging":false,"externalPower":true,"complete":true}`, 0x1b, 0x2a},
	} {
		s, err := Decode(tc.json)
		if err != nil {
			t.Fatal(err)
		}
		if s.DualShock4() != tc.ds4 || s.DualSense() != tc.ds5 {
			t.Fatalf("%s: %x %x", tc.json, s.DualShock4(), s.DualSense())
		}
	}
}
func TestStatusRejectsMalformedSnapshots(t *testing.T) {
	for _, payload := range []string{
		`{}`, `null`, `[]`, `{"version":1}`, `{"version":1,"percent":101,"charging":false,"externalPower":false,"complete":false}`,
		`{"version":2,"percent":10,"charging":false,"externalPower":false,"complete":false}`,
		`{"version":1,"percent":10,"charging":true,"externalPower":false,"complete":false}`,
		`{"version":1,"percent":10,"charging":false,"externalPower":false,"complete":true}`,
		`{"version":1,"percent":0,"charging":null,"externalPower":false,"complete":false}`,
		`{"version":1,"percent":0,"charging":false,"externalPower":false,"complete":false,"percent":100}`,
		`{"version":1,"percent":0,"charging":false,"externalPower":false,"complete":false} {}`,
	} {
		if _, err := Decode(payload); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
}
