package mouse

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/usbip"
)

func TestIdlePreservesButtonsWithoutRepeatingRelativeDeltas(t *testing.T) {
	m, _ := New(nil)
	m.HandleTransfer(context.Background(), 1, usbip.DirIn, nil)
	for _, next := range []InputState{{Buttons: 3, DX: 50, DY: -10, Wheel: 2, Pan: -3}, {}} {
		m.UpdateInputState(next)
		if got := m.HandleTransfer(context.Background(), 1, usbip.DirIn, nil); !bytes.Equal(got, next.BuildReport()) {
			t.Fatalf("update: %v", got)
		}
		for i := 0; i < 3; i++ {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			got := m.HandleTransfer(ctx, 1, usbip.DirIn, nil)
			cancel()
			idle := InputState{Buttons: next.Buttons}
			if !bytes.Equal(got, idle.BuildReport()) {
				t.Fatalf("idle repeated movement or stale buttons: %v", got)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m.HandleTransfer(ctx, 1, usbip.DirIn, nil) != nil {
		t.Fatal("cancellation produced input")
	}
}
