package genericjoystick

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/internal/inputpresentation"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
)

type CreateOptions struct {
	Version      int     `json:"version"`
	Experimental bool    `json:"experimental"`
	ButtonCount  int     `json:"buttonCount"`
	NeutralAxes  []int16 `json:"neutralAxes,omitempty"`
}

type Joystick struct {
	mu                 sync.Mutex
	input              *inputpresentation.FixedReportScheduler[InputState]
	producer           inputpresentation.FixedReportProducerLease
	producerActive     bool
	neutral, presented InputState
	pending            *InputState
	pendingAt          time.Time
	version            uint64
	buttonCount        int
	descriptor         usb.Descriptor
}

func New(o *device.CreateOptions) (*Joystick, error) {
	if o == nil {
		return nil, fmt.Errorf("genericjoystick requires explicit experimental version 1 options")
	}
	var args CreateOptions
	decoder := json.NewDecoder(strings.NewReader(o.DeviceSpecific))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return nil, fmt.Errorf("invalid genericjoystick options: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("genericjoystick options must contain exactly one JSON object")
	}
	if args.Version != ContractVersion || !args.Experimental {
		return nil, fmt.Errorf("genericjoystick requires experimental=true and version=1")
	}
	if args.ButtonCount != 12 && args.ButtonCount != 16 {
		return nil, fmt.Errorf("buttonCount must be 12 or 16")
	}
	if o.IDVendor != nil || o.IDProduct != nil {
		return nil, fmt.Errorf("experimental joystick identity cannot be overridden")
	}
	neutral := *NewInputState()
	if len(args.NeutralAxes) != 0 {
		if len(args.NeutralAxes) != 6 {
			return nil, fmt.Errorf("neutralAxes must contain six axes in X/Y/Z/Rx/Ry/Rz order")
		}
		neutral.X, neutral.Y, neutral.Z = args.NeutralAxes[0], args.NeutralAxes[1], args.NeutralAxes[2]
		neutral.Rx, neutral.Ry, neutral.Rz = args.NeutralAxes[3], args.NeutralAxes[4], args.NeutralAxes[5]
	}
	if err := neutral.Validate(args.ButtonCount); err != nil {
		return nil, err
	}
	input, err := inputpresentation.NewFixedReportSchedulerWithMaximumOrderedAge(
		ReportSize, neutral, func(s *InputState, b []byte) int { return s.BuildReportInto(b) },
		func(a, b InputState) bool { return a.Buttons != b.Buttons || a.Hat != b.Hat },
		50*time.Millisecond, time.Now())
	if err != nil {
		return nil, err
	}
	return &Joystick{input: input, neutral: neutral, presented: neutral, version: 1,
		buttonCount: args.ButtonCount, descriptor: makeDescriptor(args.ButtonCount)}, nil
}

func (j *Joystick) GetDescriptor() *usb.Descriptor { return &j.descriptor }
func (j *Joystick) VIIPERDeviceType() string       { return "genericjoystick" }
func (j *Joystick) GetDeviceSpecificArgs() map[string]any {
	return map[string]any{"version": ContractVersion, "experimental": true, "buttonCount": j.buttonCount,
		"neutralAxes": []int16{j.neutral.X, j.neutral.Y, j.neutral.Z, j.neutral.Rx, j.neutral.Ry, j.neutral.Rz}}
}

func (j *Joystick) acquireProducer() (inputpresentation.FixedReportProducerLease, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.producerActive {
		return inputpresentation.FixedReportProducerLease{}, false
	}
	j.producer = j.input.ProducerLease()
	j.producerActive = true
	return j.producer, true
}

func (j *Joystick) releaseProducer(lease inputpresentation.FixedReportProducerLease) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.producerActive || lease != j.producer {
		return
	}
	j.input.RetireProducerLease(lease, time.Now())
	j.producerActive = false
	j.pending = nil
	j.presented = j.neutral
	j.version++
}

func (j *Joystick) publish(lease inputpresentation.FixedReportProducerLease, s InputState, at time.Time) error {
	if err := s.Validate(j.buttonCount); err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.producerActive || lease != j.producer {
		return fmt.Errorf("stale genericjoystick producer")
	}
	disposition := j.input.PublishWithLease(lease, s, at)
	if disposition.Accepted() {
		return nil
	}
	switch disposition {
	case inputpresentation.FixedReportPublishRejectedNeutralPending, inputpresentation.FixedReportPublishRejectedResynchronizationRequired:
		j.pending = &s
		j.pendingAt = at
		j.tryResynchronizeLocked()
		return nil
	default:
		// Overflow/stale history closes the feeder, fences it with neutral, and
		// requires a new stream rather than quietly losing ordered button edges.
		return fmt.Errorf("genericjoystick input history fault: %v", disposition)
	}
}

func (j *Joystick) tryResynchronizeLocked() {
	if j.pending != nil && j.producerActive && j.input.Resynchronize(j.producer, *j.pending, j.pendingAt).Accepted() {
		j.pending = nil
	}
}

func (j *Joystick) ClaimInputPresentation(b []byte, at time.Time) inputpresentation.Claim {
	j.mu.Lock()
	defer j.mu.Unlock()
	claim := j.input.ClaimInputPresentation(b, at)
	j.fenceSnapshotLocked()
	return claim
}
func (j *Joystick) OwnsInputPresentationEndpoint(ep uint8) bool { return ep == 1 }
func (j *Joystick) InputPresentationGeneration() uint64         { return j.input.Generation() }
func (j *Joystick) CanAdmitInputPresentation(c inputpresentation.Claim, at time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	accepted := j.input.CanAdmitInputPresentation(c, at)
	j.fenceSnapshotLocked()
	return accepted
}
func (j *Joystick) ResolveInputPresentation(c inputpresentation.Claim, outcome inputpresentation.Outcome, at time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	state, valid := j.input.StateForInputPresentationClaim(c)
	resolved := j.input.ResolveInputPresentation(c, outcome, at)
	if resolved && valid && outcome == inputpresentation.OutcomeCommit && !j.input.Snapshot().MandatoryNeutral {
		j.presented = state
		j.version++
		j.tryResynchronizeLocked()
	}
	j.fenceSnapshotLocked()
	return resolved
}
func (j *Joystick) RetireInputPresentationGeneration(g uint64, at time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	retired := j.input.RetireInputPresentationGenerationWithBaseline(g, j.neutral, at)
	if retired {
		j.producerActive = false
		j.pending = nil
		j.presented = j.neutral
		j.version++
	}
	return retired
}
func (j *Joystick) fenceSnapshotLocked() {
	if j.input.Snapshot().MandatoryNeutral && j.presented != j.neutral {
		j.presented = j.neutral
		j.version++
	}
}
func (j *Joystick) SupportsInputReportSnapshot(id uint8) bool { return id == ContractVersion }
func (j *Joystick) SnapshotInputReportInto(b []byte) (int, uint64) {
	return j.SnapshotInputReportForIDInto(ContractVersion, b)
}
func (j *Joystick) SnapshotInputReportForIDInto(id uint8, b []byte) (int, uint64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.fenceSnapshotLocked()
	if !j.SupportsInputReportSnapshot(id) {
		return 0, j.version
	}
	return j.presented.BuildReportInto(b), j.version
}
func (j *Joystick) InputReportSnapshotCurrent(v uint64) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.fenceSnapshotLocked()
	return v != 0 && v == j.version
}
func (j *Joystick) HandleTransfer(ctx context.Context, ep, dir uint32, _ []byte) []byte {
	if dir != usbip.DirIn || ep != 1 || ctx.Err() != nil {
		return nil
	}
	b := make([]byte, ReportSize)
	c := j.ClaimInputPresentation(b, time.Now())
	if !c.Valid() {
		return nil
	}
	if ctx.Err() != nil || !j.CanAdmitInputPresentation(c, time.Now()) {
		j.ResolveInputPresentation(c, inputpresentation.OutcomeDefer, time.Now())
		return nil
	}
	if !j.ResolveInputPresentation(c, inputpresentation.OutcomeCommit, time.Now()) {
		return nil
	}
	return b
}

var _ inputpresentation.AdmissionSource = (*Joystick)(nil)
