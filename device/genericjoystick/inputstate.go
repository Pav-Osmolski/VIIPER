// Package genericjoystick implements the experimental, input-only legacy joystick.
package genericjoystick

import (
	"encoding/binary"
	"fmt"
)

const (
	ContractVersion = 1
	ReportSize      = 16
	AxisMaximum     = 32767
	HatNeutral      = 8
)

// InputState uses named DirectInput axis roles, not stick/trigger report indices.
// Signed wire axes are -32767..32767, with exactly zero at centre. HID uses
// 0..65534 so the same centre is exactly representable. Hat 0..7 is clockwise
// from north; 8 is neutral. The first byte is a wire version / HID report ID.
// viiper:wire genericjoystick c2s version:u8 x:i16 y:i16 z:i16 rx:i16 ry:i16 rz:i16 buttons:u16 hat:u8
type InputState struct {
	X, Y, Z, Rx, Ry, Rz int16
	Buttons             uint16
	Hat                 uint8
}

func NewInputState() *InputState { return &InputState{Hat: HatNeutral} }

func (s InputState) Validate(buttonCount int) error {
	for _, axis := range []int16{s.X, s.Y, s.Z, s.Rx, s.Ry, s.Rz} {
		if axis == -32768 {
			return fmt.Errorf("axis must be from -32767 through 32767")
		}
	}
	if s.Hat > HatNeutral {
		return fmt.Errorf("hat must be from 0 through 8")
	}
	if buttonCount != 12 && buttonCount != 16 {
		return fmt.Errorf("button count must be 12 or 16")
	}
	if buttonCount == 12 && s.Buttons&0xF000 != 0 {
		return fmt.Errorf("buttons exceed the 12-button descriptor")
	}
	return nil
}

func (s InputState) MarshalBinary() ([]byte, error) {
	if err := s.Validate(16); err != nil {
		return nil, err
	}
	b := make([]byte, ReportSize)
	b[0] = ContractVersion
	for i, axis := range []int16{s.X, s.Y, s.Z, s.Rx, s.Ry, s.Rz} {
		binary.LittleEndian.PutUint16(b[1+i*2:], uint16(axis))
	}
	binary.LittleEndian.PutUint16(b[13:], s.Buttons)
	b[15] = s.Hat
	return b, nil
}

func (s *InputState) UnmarshalBinary(data []byte) error {
	if len(data) != ReportSize {
		return fmt.Errorf("generic joystick frame must be exactly %d bytes", ReportSize)
	}
	if data[0] != ContractVersion {
		return fmt.Errorf("unsupported generic joystick wire version %d", data[0])
	}
	// Validate before publishing, leaving the receiver unchanged on failure.
	next := InputState{
		X: int16(binary.LittleEndian.Uint16(data[1:])), Y: int16(binary.LittleEndian.Uint16(data[3:])),
		Z: int16(binary.LittleEndian.Uint16(data[5:])), Rx: int16(binary.LittleEndian.Uint16(data[7:])),
		Ry: int16(binary.LittleEndian.Uint16(data[9:])), Rz: int16(binary.LittleEndian.Uint16(data[11:])),
		Buttons: binary.LittleEndian.Uint16(data[13:]), Hat: data[15],
	}
	if err := next.Validate(16); err != nil {
		return err
	}
	*s = next
	return nil
}

func (s InputState) BuildReportInto(destination []byte) int {
	if len(destination) < ReportSize || s.Validate(16) != nil {
		return 0
	}
	destination[0] = ContractVersion
	for i, axis := range []int16{s.X, s.Y, s.Z, s.Rx, s.Ry, s.Rz} {
		binary.LittleEndian.PutUint16(destination[1+i*2:], uint16(int(axis)+AxisMaximum))
	}
	binary.LittleEndian.PutUint16(destination[13:], s.Buttons)
	destination[15] = s.Hat
	return ReportSize
}
