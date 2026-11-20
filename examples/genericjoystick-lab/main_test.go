package main

import "testing"

func TestFrameValidationBeforePublication(t *testing.T) {
	valid := "01000000000000000000000000000008"
	if state, err := decodeFrame(valid, 12); err != nil || state.Hat != 8 {
		t.Fatal(state, err)
	}
	for _, bad := range []string{"", valid + "00", "02000000000000000000000000000008", "01008000000000000000000000000008", "01000000000000000000000000001008", "01000000000000000000000000000009"} {
		if _, err := decodeFrame(bad, 12); err == nil {
			t.Fatalf("accepted malformed frame %s", bad)
		}
	}
}
