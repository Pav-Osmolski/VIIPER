package genericjoystick

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/usb"
)

func init() { api.RegisterDevice("genericjoystick", &handler{}) }

type handler struct{}

func (*handler) CreateDevice(o *device.CreateOptions) (usb.Device, error) { return New(o) }
func (*handler) UpdateMetaState(_ string, _ *usb.Device) error {
	return fmt.Errorf("genericjoystick has no metadata commands")
}
func (*handler) StreamHandler() api.StreamHandlerFunc {
	return func(conn net.Conn, ptr *usb.Device, _ *slog.Logger) error {
		if ptr == nil || *ptr == nil {
			return fmt.Errorf("nil genericjoystick device")
		}
		j, ok := (*ptr).(*Joystick)
		if !ok {
			return fmt.Errorf("device is not genericjoystick")
		}
		lease, acquired := j.acquireProducer()
		if !acquired {
			return fmt.Errorf("genericjoystick producer is already connected")
		}
		defer j.releaseProducer(lease)
		var frame [ReportSize]byte
		for {
			if _, err := io.ReadFull(conn, frame[:]); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			var state InputState
			if err := state.UnmarshalBinary(frame[:]); err != nil {
				return err
			}
			if err := j.publish(lease, state, time.Now()); err != nil {
				return err
			}
		}
	}
}
