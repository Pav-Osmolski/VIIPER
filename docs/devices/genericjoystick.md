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

These are reference behaviours, **not verified compatibility claims for those
games**. The native Windows synthetic-input lab passed on 2026-10-14. A USB
DualSense physical preview and two games were subsequently tested; see the
dated results below. The games named in the original feature request remain
unverified.

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

The DS4Windows feature branch now supplies an isolated Windows lab launcher in
`utils/GenericJoystickLab`. Its interactive feeder is this repository's
`examples/genericjoystick-lab`: authenticated literal-loopback API, explicit
absolute key/options files, allocation and attachment receipts, bounded
management requests, validated versioned frames, producer disconnect/reconnect,
and bus cleanup on exit. It cannot install drivers or detach native imports;
the launcher owns that operation using the private server/bus/device identity.

The packaged lab checks both stick layouts and all four trigger modes against
the actual DS4Windows mapping policy, using x86/x64 DirectInput and WinMM.
It preserves released app files and excludes the API password from reports.
The launcher has simulated success, timeout, failed-stream and foreign-import
tests. Revision 4's native Windows report passed all 156 observations across
eight layout/trigger plans, x86/x64 DirectInput and WinMM, configured neutral,
producer disconnect and reconnect. Temporary imports were removed without
cleanup errors. Simulated socket tests alone are not evidence of enumeration;
this result comes from the user-run USB/IP 0.9.8.1 native attachment lab on
Windows 10.0.26300.0. It validates the twelve-button synthetic configuration on
one machine, not physical-controller integration, sixteen-button enumeration,
force feedback or game compatibility.

Report: `20261014-233817-ea0f280a-report.zip`. Tested VIIPER revision:
`01801558b9c52616afaf145290067c57ad2cbeb3`; matching DS4Windows revision:
`4e158868809eaa3c60ddb014bb2a4fc3da2e1ea1`. Detailed results are recorded in the
frontend branch's `docs/directinput-development.md`.

The synthetic lab established the native baseline at its recorded revisions.
Subsequent physical-preview evidence is recorded below. Remaining release work
is tracked in the frontend's
[release checklist](https://github.com/Pav-Osmolski/DS4Windows/blob/feature/directinput-output/docs/directinput-release-checklist.md):
production identity, normal-mode integration and built-in HidHide validation,
then matched release packaging. NFS Underground 2, Jedi Outcast/Academy and
Serious Sam TFE/TSE still need tests recording edition, patches, bindings and
controller configuration. Force feedback and arbitrary descriptor customization
are separate future work.

## DS4Windows portable physical-controller preview

The corresponding DS4Windows feature branch now exposes this output only in its
hash-pinned portable lab context. It sends the requested six resting axes during
creation and waits for the versioned `genericjoystick-v1` capability response and
`inputConfigured: true` before opening the producer. The backend reports readiness
only after an accepted USB SET_CONFIGURATION(1). Configuration zero and exact
native import closure clear readiness; stale import generations cannot clear a
successor's state. Resetting an endpoint alone does not establish configuration.

The physical preview uses twelve buttons and the validated wire contract. It
adds no feedback, audio or production identity allocation. See DS4Windows's
`utils/GenericJoystickPreview/README.txt` for the isolated launcher and test plan.

### Owner-run results, 2026-10-15

The owner confirmed all eight layout/trigger combinations with a USB DualSense,
saved settings, inversions/button thresholds, unplug/reconnect and Stop/Start.
The r2 preview paired VIIPER `bd8f6e21215b471015ca442945c2c7fc90f1cab4`
with DS4Windows `6c2760c100f04e515ec7458c0b7db565ff26c8e9`.

| Game | DS4Windows configuration | User result |
| --- | --- | --- |
| BeamNG.drive 0.39.4.0.20972 | Racing + Full range; X steering, Rz throttle, Z brake | Hardware response, bindings and driving passed. |
| FlatOut 2, current Steam installation; exact build unrecorded | Legacy + Centred; X steering, Ry throttle, Rx brake | Driving with manual and automatic transmission passed. Stick menu navigation felt awkward at 0% in-game dead zone. |

The physical matrix report `20261015-123537-0ff0761f-report.zip` recorded fifteen
attachments/removals and clean cleanup. The play-session report
`20261015-124824-f68ac364-report.zip` recorded 3,570 sampled input states,
three closed sessions, no backend WARN/ERROR events and no remaining imports.
Gameplay success is the owner's observation; backend logs cannot prove it.
These results cover one machine and the tested game installations, not all
versions or original unpatched releases. Steam Input/HidHide state was not
independently captured. The preview deliberately bypasses automatic HidHide.

### Production identity preparation

A [pid.codes request](https://github.com/pidcodes/pidcodes.github.com/pull/1317)
was submitted on 2026-10-15 for one twelve-button descriptor and is **pending
approval**. Its proposed
`1209:D10C` must not be used until assigned. The sixteen-button variant remains
lab-only, needs native validation and must have a separate identity. Keep normal
DS4Windows activation disabled until identity and matched packaging are ready.

### Optional stable instance identity

Creation accepts `instanceId`: exactly 32 hexadecimal digits, normalized to
uppercase and echoed in device-specific metadata. USB serials are now
`VIIPER-GJ1-<instanceId>`. Omitting the field generates a random 128-bit identity,
so independent default devices do not share a serial. To preserve identity on
recreation, clients must supply the same ID for the same logical controller and
different IDs for different controllers. The backend does not globally reserve
caller-supplied IDs; callers must avoid duplicates. DS4Windows rejects duplicate
physical identities and verifies the returned ID before opening/reopening input.
The HID report layout and preview VID/PIDs are unchanged. A new serial can require
rebinding existing games. Stable serials do not guarantee stable WinMM slot
numbers or game-specific saved bindings. The completed two-device owner lab (r4 report `20261015-165810-0ed8a66d`)
validated input isolation, producer reconnect, recreation and reversed creation
order in x86/x64 DirectInput and WinMM. It retained two GUID-persistence failures:
removing A moved B from joystick slot 1 to 0 and changed B's GUID in enumeration
and GetDeviceInfo. B's serial/path stayed stable; restoring A restored the
original GUIDs. Cleanup was clean. Stable multi-controller game bindings remain
unverified.

The USB/IP 0.9.8.1 serial-override fix
[a2a1159](https://github.com/vadimgrn/usbip-win2/commit/a2a1159763088c85d940078cfa6491f38540953d)
was not in a published successor release as of 2026-10-15. VIIPER leaves the
transport override empty and serves its own serial, but later serial-query
success does not establish correct initial Windows PnP enumeration. Retest the
same two-device scenario when a packaged fixed driver is available before
ruling out an effect from its shared request-completion changes.
