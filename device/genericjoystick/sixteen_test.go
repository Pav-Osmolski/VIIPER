package genericjoystick

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestSixteenButtonPressReleaseAndProducerLoss(t *testing.T) {
	j := newTestJoystick(t, 16)
	if j.GetDeviceSpecificArgs()["buttonCount"] != 16 {
		t.Fatal("wrong capability")
	}
	lease, _ := j.acquireProducer()
	at := time.Now()
	for button := 0; button < 16; button++ {
		mask := uint16(1 << button)
		if err := j.publish(lease, InputState{Buttons: mask, Hat: HatNeutral}, at); err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint16(present(t, j, at)[13:]); got != mask {
			t.Fatalf("button %d: %04x", button+1, got)
		}
		at = at.Add(time.Microsecond)
		if err := j.publish(lease, InputState{Hat: HatNeutral}, at); err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint16(present(t, j, at)[13:]); got != 0 {
			t.Fatalf("button %d stuck", button+1)
		}
		at = at.Add(time.Microsecond)
	}
	if err := j.publish(lease, InputState{Buttons: 0xF000, Hat: HatNeutral}, at); err != nil {
		t.Fatal(err)
	}
	present(t, j, at)
	j.releaseProducer(lease)
	if got := binary.LittleEndian.Uint16(present(t, j, time.Now())[13:]); got != 0 {
		t.Fatalf("extra buttons stuck after producer loss: %04x", got)
	}
}
