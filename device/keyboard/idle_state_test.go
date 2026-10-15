package keyboard

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/usbip"
)

func TestIdleMaintainsLatestKeysAndRelease(t *testing.T) {
	k, _ := New(nil)
	k.HandleTransfer(context.Background(), 1, usbip.DirIn, nil)
	state := InputState{Modifiers: 2}
	state.KeyBitmap[0] = 16
	for _, next := range []InputState{state, {}} {
		k.UpdateInputState(next)
		if got := k.HandleTransfer(context.Background(), 1, usbip.DirIn, nil); !bytes.Equal(got, next.BuildReport()) {
			t.Fatalf("update: %v", got)
		}
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		got := k.HandleTransfer(ctx, 1, usbip.DirIn, nil)
		cancel()
		if !bytes.Equal(got, next.BuildReport()) {
			t.Fatalf("idle: %v", got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if k.HandleTransfer(ctx, 1, usbip.DirIn, nil) != nil {
		t.Fatal("cancellation produced input")
	}
}
