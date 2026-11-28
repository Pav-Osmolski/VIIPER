package dualshock4

import (
	"context"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/usbip"
)

func TestShortPressAndReleaseSurviveBetweenHostReads(t *testing.T) {
	d, err := New(&device.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	press, release := NewInputState(), NewInputState()
	press.Buttons = ButtonCross
	d.UpdateInputState(press)
	d.UpdateInputState(release)
	release.LX = 95
	d.UpdateInputState(release)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for i, want := range []struct {
		cross bool
		lx    byte
	}{{true, 128}, {false, 128}, {false, 223}} {
		report := d.HandleTransfer(ctx, EndpointIn, usbip.DirIn, nil)
		if len(report) != InputReportSize || (report[5]&0x20 != 0) != want.cross || report[1] != want.lx {
			t.Fatalf("incorrect ordered HID report %d: %x", i, report)
		}
	}
}

func TestReplacedStreamAndPausedHostCannotReplayOldPress(t *testing.T) {
	q := dualShock4InputQueue{ready: make(chan struct{}, 1)}
	now := time.Now()
	press, release := *NewInputState(), *NewInputState()
	press.Buttons = ButtonCross

	older, newer := &dualShock4SerialReservation{}, &dualShock4SerialReservation{}
	q.beginStream(older)
	q.publish(press, now, older)
	q.beginStream(newer)
	q.publish(press, now, older)
	q.endStream(older)
	if state, ok := q.take(now, false); !ok || state.Buttons != 0 {
		t.Fatal("a replaced stream kept a press alive")
	}

	q.publish(press, now, newer)
	q.publish(release, now, newer)
	if state, ok := q.take(now.Add(maximumInputTransitionAge), false); !ok || state.Buttons != 0 {
		t.Fatal("a paused host replayed an expired press")
	}
}
