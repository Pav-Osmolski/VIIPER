// Package batterystatus defines a narrow, versioned power-status control plane.
// It never changes the fixed-size controller input-state stream.
package batterystatus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type StatusV1 struct {
	Version       uint16 `json:"version"`
	Percent       uint8  `json:"percent"`
	Charging      bool   `json:"charging"`
	ExternalPower bool   `json:"externalPower"`
	Complete      bool   `json:"complete"`
}

func Decode(payload string) (StatusV1, error) {
	var s StatusV1
	if len(payload) > 512 {
		return s, errors.New("battery payload too large")
	}
	d := json.NewDecoder(strings.NewReader(payload))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return s, errors.New("expected battery object")
	}
	seen := map[string]bool{}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return s, err
		}
		key, ok := t.(string)
		if !ok || seen[key] {
			return s, errors.New("duplicate battery field")
		}
		seen[key] = true
		switch key {
		case "version":
			err = d.Decode(&s.Version)
		case "percent":
			err = d.Decode(&s.Percent)
		case "charging":
			err = d.Decode(&s.Charging)
		case "externalPower":
			err = d.Decode(&s.ExternalPower)
		case "complete":
			err = d.Decode(&s.Complete)
		default:
			return s, fmt.Errorf("unknown battery field %q", key)
		}
		if err != nil {
			return s, err
		}
	}
	if _, err = d.Token(); err != nil {
		return s, err
	}
	if d.Decode(new(any)) != io.EOF {
		return s, errors.New("trailing battery data")
	}
	if len(seen) != 5 || s.Version != 1 || s.Percent > 100 ||
		(s.Charging && !s.ExternalPower) ||
		(s.Complete && (!s.ExternalPower || s.Charging || s.Percent != 100)) {
		return s, errors.New("invalid or incomplete battery status v1")
	}
	// Typed Decode accepts null as a no-op. Require explicit non-null fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return s, err
	}
	for _, v := range raw {
		if string(v) == "null" {
			return s, errors.New("null battery field")
		}
	}
	return s, nil
}

// DualShock4 encodes discrete ten-percent bands rather than an exact percentage.
func (s StatusV1) DualShock4() byte {
	level := s.Percent / 10
	if s.Complete {
		level = 11
	}
	if s.ExternalPower {
		level |= 0x10
	}
	return level
}

func (s StatusV1) DualSense() byte {
	level := s.Percent / 10
	if s.Complete {
		return level | 0x20
	}
	if s.Charging {
		return level | 0x10
	}
	return level
}
