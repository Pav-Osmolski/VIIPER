# October 2026 upstream integration

Reviewed Alia5 main `ef817d51ca1175c86c6c485c0bf52822c5d6ae49` against fork
main `a7ad40ea79b9f44785675f6c70dd3f0d3e3fd4c8` and DS4Windows 5.0.24
(`04065ce23fb5eb9a54275c38070ac22054b34792`). Common ancestor:
`88f66f1ed0c3716c78f810d92b1924112093f896`.

## Adapted improvements

| Upstream | Fork integration |
| --- | --- |
| `bfe2bb7` acceleration constants | Match the fork's existing calibration; derive the conversion factor from 8192 counts/g and standard gravity rather than rounding it. Raw motion packet fields are unchanged. |
| `674d9cc` keyboard/mouse idle state | Retain the latest absolute state under the existing publication mutex. Return it only on a poll deadline, with mouse deltas zeroed; explicit cancellation still returns nil. Remove the old mouse zero-report requeue, which could race a producer. Retain the existing 5 ms descriptor cadence. |
| `ef817d5` no-tray flag | Optional tray creation with the existing cancellation and stop lifecycle; prerequisite checks, key file handling and listener ownership are unchanged. |
| `68d77e9` DS4 output validity | Resolve partial USB commands in the broker under the device lock, and commit only after framed admission succeeds. Keep the seven-byte complete-state wire format and generated-client schema. Existing DS4Windows versions require no protocol change. |
| `8420705`, `eff0f95`, `28f25c9`, `e64e22f` dependencies/toolchain | Adopt Go 1.27.2 and the updated versions of libraries already used by the fork. Keep the fork's newer kong-yaml revision; do not add unused TUI, external tray or updater dependencies. Update CI and provenance toolchain checks together. |

DS4 validity bits are 0x01 for rumble, 0x02 for RGB and 0x04 for flash. Clients
continue receiving complete state; the wire protocol does not expose the
original per-command mask. This intentionally does not provide upstream's new
eight-byte callback ABI. Synthetic USB producers must set the appropriate bits;
a zero mask leaves all values unchanged. Other device protocols are unchanged.

## Benefits already provided by fork-specific implementations

| Upstream | Existing fork behaviour |
| --- | --- |
| `41c66b1`, `a7471ed`, `32ee820` USB/IP compatibility | Explicit 0.9.8.1 native ABI layouts, bounded prerequisite checks and attach/cancellation handling. |
| `6a5602a` OUT reply correctness | The USB/IP writer already clears OUT response data. |
| `d5a672f` disconnected-device cleanup | Scheduler cancellation closes the connection and schedules empty-bus cleanup. This does not concern Windows driver-package uninstallation. |
| `3111299` no-data CPU spin | Persistent endpoint workers and timed scheduling replace the old per-request loop. |
| `c8df571`, `24a1a2a` installation/updater | DS4Windows owns prerequisite repair, startup, deployment and updates. Standalone mutation remains explicitly restricted. Retain this ownership instead of adding a competing installer/updater. |

## Distribution and mechanical changes

Upstream signing (`0decd2a`) uses Alia5's identity and signing infrastructure.
This fork cannot inherit its certificate by merging code. No fork signing
credentials were configured at review time. An owner-controlled certificate or
signing service must be configured separately before Authenticode signing can
be implemented and verified. Do not substitute an upstream binary for this
fork's customised broker.

Upstream WinGet work (`e18632f`, `b3cfcce`) and archive removal (`357df54`) are
not imported: package publication requires a fork-specific identifier, immutable
published artifacts and verified checksums. DS4Windows users should keep using
the matching managed package. Preserve offline archives, source, notices and
provenance instead of removing assets consumed by the managed integration.

The remaining documentation/CI/client changes (`cd2e000`, `7b32294`, `a068fe4`,
`f7418ed`, `edced9e`, `52a4a8a`, `d36fb4b`, `7d6d84f`) are reviewed against
the fork's own build and generated-client contracts. A client-package rename or
upstream updater documentation does not supply an additional runtime benefit.
The fork already uses newer action major versions in several places.

## Regression coverage

- DS4 helpers/defaults versus advertised calibration.
- Partial USB rumble/RGB/flash commands through interrupt, control and framed
  routes; zero validity masks; exact seven-byte feedback; reconnect replay;
  rejected admission must not leak into the next accepted partial command.
- Keyboard held keys and releases across idle polls, and explicit cancellation.
- Mouse buttons, movement and both wheels across repeated idle polls; explicit
  cancellation must not produce input.
- CLI/environment parsing and default/disabled tray lifecycle.
- Existing complete Go suite, Release-tag suite, USB scheduler and startup/
  prerequisite coverage, plus DS4Windows VIIPER integration tests.

These are automated checks. Physical controller/game validation is still
needed before a new paired DS4Windows/VIIPER release is described as hardware
validated. This source change does not replace the executable already bundled
by DS4Windows; a future paired package must update the version, binary, source,
notices, hash and provenance together.
