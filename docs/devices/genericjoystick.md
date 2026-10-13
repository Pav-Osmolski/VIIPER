# Experimental generic joystick v1

This feature branch contains an **input-only laboratory prototype**, not a
released DS4Windows output. Its matching frontend branch is
`Pav-Osmolski/DS4Windows:feature/directinput-output`.

## Purpose and reference behaviour

Older Windows games can read different axis roles and resting positions from
modern gamepads. In particular, eVenent's PadForge configuration used three
virtual sticks (six axes), no dedicated triggers, and 12 buttons. PadForge's
reported fix put the second stick on Z/Rz rather than Rx/Ry:

- [Original report](https://github.com/hifihedgehog/PadForge/discussions/122)
- [Reported root cause and fix](https://github.com/hifihedgehog/PadForge/issues/123)
- [Microsoft's trigger-centre explanation](https://learn.microsoft.com/en-us/windows/win32/xinput/xinput-and-directinput)
- [Serious Engine's WinMM input implementation](https://github.com/Croteam-official/Serious-Engine/blob/master/Sources/Engine/Base/Input.cpp)

These are reference behaviours, **not verified compatibility claims for this
prototype**. Windows enumeration and original-game tests remain pending.

## Device and contract

Management type: `genericjoystick`. Creation requires:

```json
{
  "type": "genericjoystick",
  "deviceSpecific": {
    "version": 1,
    "experimental": true,
    "buttonCount": 12,
    "neutralAxes": [0, 0, 0, 0, 0, 0]
  }
}
```

Only 12 and 16 buttons are supported. `neutralAxes` is optional and defaults
to six centred axes. A feeder using full-range pedals must supply the actual
released-axis values at creation so startup, recovery and disconnect all use
the same baseline. Unknown options, unsupported versions and identity
overrides are rejected.

The descriptor has one Joystick application collection, six named absolute
axes (X, Y, Z, Rx, Ry, Rz), one eight-direction POV hat with a null position,
and one interrupt-IN endpoint. It declares no force-feedback or audio
interfaces. It is not an XInput controller.

The experimental identities are **FFFF:D10C** (12 buttons) and **FFFF:D110**
(16 buttons). These are unallocated laboratory identities, not assigned USB
IDs; settle an appropriate production identity before shipping. The different
PIDs prevent the two button-count descriptors sharing a cached Windows identity.
The prototype does not impersonate vJoy, Sony, Logitech or Mad Catz hardware.

Both stream frame and HID report are 16 bytes, but their axis value spaces
are intentionally different:

| Byte offset | Stream | HID report |
| --- | --- | --- |
| 0 | Version 1 | Report ID 1 |
| 1, 3, 5, 7, 9, 11 | Six little-endian signed 16-bit axes, -32767..32767 | Same named axes, unsigned 0..65534 |
| 13..14 | Little-endian button mask | Same mask; upper four bits are constant padding in the 12-button descriptor |
| 15 | Hat 0..7 clockwise from north; 8 neutral | Same hat in low nibble; constant upper nibble |

Wire zero is exactly HID 32767. Wire -32768, invalid hats, incorrect lengths,
unsupported versions and out-of-layout buttons are rejected before publication.
Named axis roles are stable regardless of frontend layout preset. Report ID 1
GET_REPORT uses a versioned snapshot of committed input; it does not consume
the interrupt journal.

The existing fixed-report scheduler coalesces continuous axis motion while
preserving button/hat boundaries. It enforces a 50 ms maximum ordered age,
neutralizes faulty history and rejects stale producer leases. Stream loss
retires the producer and schedules configured neutral. Endpoint retirement
starts a neutral generation rather than replaying an old press. This prototype
does not promise force feedback, audio, advanced controller features or every
legacy game's compatibility.

## Validation tools

`go run ./examples/genericjoystick` prints a labelled sequence of wire frames
by default. **Dry-run opens no connections and creates no device.**

`--api=127.0.0.1:<lab-api-port>` explicitly connects to an isolated lab backend,
allocates its own bus/device, feeds each diagnostic step, and removes its bus
on exit. `--hold=2s` controls the duration of each step. Never point it at a
DS4Windows-managed server. The feeder does not install a driver or perform
native attach; a server configured for automatic native attach may do so.

An actual Windows check requires the prototype backend, an already verified
matching USB/IP driver, a separately configured local lab server and explicit
native attachment. Do not substitute this binary into the released app's
managed backend folder or bypass package hashes. No installer or driver
upgrade is necessary just to run the automated socket tests.

The corresponding DS4Windows branch provides a read-only native consumer:
`utils/GenericJoystickProbe`. Build/run its x86 and x64 executables to compare
DirectInput's DIJOYSTATE and WinMM's JOYINFOEX. Check all axis endpoints,
neutral, buttons, hat diagonals and disconnect. Keep driver/attach ownership
with the lab process; the probe only reads.

## Remaining stages

1. Record successful Windows HID parsing and enumeration through both APIs,
   including the 32-bit reader, and settle device identity.
2. Integrate the tested DS4Windows mapper with output lifetime, saved profiles
   and a gated editor card; verify exact matching backend capability.
3. Validate NFS Underground 2, Jedi Outcast/Academy and Serious Sam TFE/TSE,
   recording game edition, patches, bindings and controller configuration.
4. Package a matched preview and invite user testing. Force feedback and
   arbitrary descriptor customization are separate future work.
