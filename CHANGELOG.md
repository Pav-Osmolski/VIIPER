# Changelog

Curated release history inherited by the Pav-Osmolski VIIPER fork.
These records describe their original releases, not a promise that every
upstream asset is republished here. For DS4Windows, use its bundled backend.

## Unreleased

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
