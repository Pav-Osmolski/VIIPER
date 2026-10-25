# Changelog

Curated release history inherited by the Pav-Osmolski VIIPER fork.
These records describe their original releases, not a promise that every
upstream asset is republished here. For DS4Windows, use its bundled backend.

## Unreleased

- Add a read-only, versioned controller-information endpoint for the selected virtual device, with descriptor identity for all output types, actual DS4/DualSense/Edge factory reports, public Xbox One firmware/hardware versions and supported status metadata. No device writes, stream reopen or operation authority are exposed.

## [0.1.15](https://github.com/Pav-Osmolski/VIIPER/releases/tag/v0.1.15) - 2026-10-24

- Correct the DualShock 4 factory report's two 16-byte date/time fields. The time previously started one byte early, causing host tools to display `Sep 17 20211 1:34:00` instead of `Sep 17 2021 11:34:00`. Thanks to [@Jobima1st](https://github.com/Jobima1st) for following up in [DS4Windows #75](https://github.com/Pav-Osmolski/DS4Windows/issues/75).
- Add host-decoding regression coverage through actual USB GET_REPORT handlers for DualShock 4, DualSense and DualSense Edge, including custom dates, padding, version fields and short reads. Audit the other emulated controller metadata formats; no matching date/time overlap was found. Paired with DS4Windows 5.0.48.

## [0.1.14](https://github.com/Pav-Osmolski/VIIPER/releases/tag/v0.1.14) - 2026-10-24

- Gracefully finish complete one-shot management replies before closing their TCP connection, bounding the drain by time and input bytes and keeping it interruptible by server shutdown. Streaming controller/audio connections, authentication, framing and operation retries are unchanged. Thanks to [@cns00](https://github.com/cns00) for the diagnostic comparisons and successful final paired-preview testing in [DS4Windows #66](https://github.com/Pav-Osmolski/DS4Windows/issues/66), also reported in [upstream #153](https://github.com/hbashton/DS4Windows/issues/153). The reporter confirmed consistently reaching Ready on 24 October. Paired with DS4Windows 5.0.47, including its separate app/installer readiness-check correction. No hardware fault or universal cause has been established.
- The earlier isolated comparison improved from 77/400 failed requests to 3/400 with graceful closing; those diagnostic measurements are historical and do not imply every possible future connection reset is eliminated. Implementation and regression coverage: [PR #9](https://github.com/Pav-Osmolski/VIIPER/pull/9).

## [0.1.13](https://github.com/Pav-Osmolski/VIIPER/releases/tag/v0.1.13) - 2026-10-23

- Add strict, versioned live battery/power status updates for virtual DualShock 4, DualSense/Edge and NS2 Pro. Preserve controller identity, measured voltage and existing input packet formats. Paired with DS4Windows 5.0.46 after successful maintainer USB/Bluetooth testing. Thanks to [@Jobima1st](https://github.com/Jobima1st) for [DS4Windows #75](https://github.com/Pav-Osmolski/DS4Windows/issues/75) and [upstream #111](https://github.com/hbashton/DS4Windows/issues/111).
- Clarify the existing LF-terminated management reply format and finish Go, generated C# and Rust client replies at LF rather than waiting for connection close. Preserve clean-EOF compatibility and reject read errors before a complete reply. Keep JSON/API error validation and authenticated-record checks.
- Preserve split UTF-8 characters in generated C# and TypeScript management replies. Make TypeScript requests reject incomplete or failed connections instead of remaining pending, and handle errors on the authenticated socket wrapper.
- The management-reply changes preserve the server wire format and do not change drivers. Thanks to [@cns00](https://github.com/cns00) for the diagnostic reports in [DS4Windows #66](https://github.com/Pav-Osmolski/DS4Windows/issues/66) and [upstream #153](https://github.com/hbashton/DS4Windows/issues/153). The reporter's connection-reset issue remains unconfirmed.

## [0.1.11](https://github.com/Pav-Osmolski/VIIPER/releases/tag/v0.1.11) — 2026-10-15

- Paired with DS4Windows 5.0.25 after successful maintainer testing of the combined package.

- Respect DS4Windows' saved per-user tray visibility at installed Windows startup without rewriting startup tasks. Explicit CLI/environment values take priority; explicit-key portable/lab deployments remain isolated unless they pass a choice.

- Match virtual DS4 acceleration defaults and conversion helpers to the existing 8192-count/g calibration.
- Honour DS4 USB rumble, RGB and flash validity flags while preserving the seven-byte complete-state feedback protocol used by DS4Windows and existing clients. Fields not selected by a partial command retain their last accepted values.
- Retain keyboard keys and mouse buttons during idle input polls without replaying relative mouse movement or wheel deltas.
- Add optional `server --no-tray` / `VIIPER_NO_TRAY` operation, preserving normal startup ownership and tray behaviour by default.
- Update the Go toolchain and existing shared dependencies to the reviewed upstream versions. Keep DS4Windows-managed installation, updates and fork-specific USB/IP scheduling.

## [0.1.10](https://github.com/Pav-Osmolski/VIIPER/releases/tag/v0.1.10) — 2026-10-11

- Published after the maintainer successfully tested the combined DS4Windows 5.0.15 package.
- Support the published usbip-win2 0.9.8.1 Windows x64 package, including its new native attach and exact-location retry cancellation layouts.
- Use the new low-latency WSK receive mode for Windows virtual-controller imports, through both native and command-line attach paths.
- Preserve exact Xbox One registration/removal authority, one-attempt production attachment, and cancellation buffer lifetimes.
- Leave attach serial overrides empty because the published upstream 0.9.8.1 implementation has a known bug.
- Update the developer-only Windows setup pin and accept the new plain serial field while rejecting foreign imports during cleanup.
- Build numeric fork releases as drafts for testing, with corresponding source and checksums. Client registry publication remains disabled for this fork.

## Inherited release records

The following records preserve the original release descriptions and their
limitations. They are historical context, not current setup instructions.

<a id="inherited-release-notes-v016-rc464"></a>

### release-notes-v0.1.6-rc4.6.4

Original record: `docs/release-notes-v0.1.6-rc4.6.4.md`. Instructions and validation claims apply to that release.

### VIIPER 0.1.6-rc4.6.4 — Smoother Controller Feedback

This update accompanies DS4Windows RC4.6.4 and improves the consistency and detail of supported controller feedback.

#### Improvements

- Improved conversion of advanced game haptics for DualSense and DualSense Edge over Bluetooth when used with the matching DS4Windows update.
- More consistent controller-audio delivery through brief timing variations, helping prevent interrupted haptic effects.
- Backward-compatible operation with clients that do not request the updated conversion.

#### Install or update

**DS4Windows users:** install the matching **DS4Windows RC4.6.4 all-in-one installer**, or extract its complete portable ZIP. Both include the matching VIIPER build; a separate installation is not necessary.

Close your games, exit DS4Windows, and quit VIIPER before updating. Keep **USB/IP 0.9.7.7** when using DS4Windows.

Standalone users should select the archive for their operating system and architecture from the assets below. This is a release candidate.

Thanks for using VIIPER!

<a id="inherited-release-notes-v017-rc465"></a>

### release-notes-v0.1.7-rc4.6.5

Original record: `docs/release-notes-v0.1.7-rc4.6.5.md`. Instructions and validation claims apply to that release.

### VIIPER 0.1.7-rc4.6.5 — Updates Made Simple

VIIPER updates now come with DS4Windows. There are no separate VIIPER update pop-ups or automatic downloads.

Your existing settings remain compatible. Controller support and feedback are unchanged in this update.

#### How to update

**DS4Windows users:** use the matching DS4Windows RC4.6.5 installer or complete portable ZIP. Both include VIIPER, so you do not need a separate download.

Standalone users can download the archive for their operating system below. Close VIIPER before replacing its files.

This is a release candidate. Thanks for your feedback!

<a id="inherited-release-notes-v019-rc466"></a>

### release-notes-v0.1.9-rc4.6.6

Original record: `docs/release-notes-v0.1.9-rc4.6.6.md`. Instructions and validation claims apply to that release.

### VIIPER 0.1.9-rc4.6.6 — Reliable Startup & Better Edge Support

- Improved startup and shutdown cleanup so failed or canceled attempts can be retried more reliably.
- Added clearer startup failure information for DS4Windows.
- Improved virtual DualSense Edge reports and compatibility with the shared DualSense feedback path.
- Unsupported Edge onboard-profile commands now fail clearly instead of appearing to succeed.

**DS4Windows users:** use the matching RC4.6.6 installer or complete portable ZIP. Both include the correct VIIPER, so no separate update is needed.

Standalone users can download the archive for their operating system below. Close VIIPER before replacing its files.

This is a release candidate. Virtual Edge onboard-profile editing is not supported. Thanks for your feedback!
