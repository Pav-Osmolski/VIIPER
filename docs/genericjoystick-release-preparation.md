# Generic Joystick release preparation

The standard local DS4Windows 5.0.84 packages bundle VIIPER 0.1.19 and use
the registered sixteen-button device. Automated checks cover all sixteen
button transitions, producer-loss neutralization, and distinct reports and
stable USB serials across two registered device lifetimes. Native game binding,
Windows DirectInput GUID retention and installed acceptance remain open.

## Reuse the existing release pipeline

1. The standard sixteen-button build uses [the allocated USB identity 1209:D10C](https://pid.codes/1209/D10C/).
   The twelve-button `FFFF:D10C` and sixteen-button `FFFF:D110` remain lab identities.
2. Review the feature diff against main and choose an explicit version under
   the existing release policy. Build from the reviewed, clean source commit
   through the release workflow, preserving release tags, trimpath, Windows
   amd64, CGO disabled and the pinned Go toolchain.
3. Retain the Windows executable, BUILD-INFO receipt, all five licence/notice
   files, corresponding-source ZIP and release checksums. Confirm receipt
   version/commit, embedded Go revision, PE versions and source archive commit
   agree. A regenerated sidecar alone is insufficient provenance.
4. In DS4Windows, run `utils/prepare_viiper_promotion.py` against those exact
   inputs. It reads binary metadata without launching VIIPER and produces a
   review-only list of coordinated pin files and hashes. Independently verify
   the release tag/source; the tool checks consistency, not reproducible-build
   equivalence or approval of USB identity.
5. Promote the application, setup/bootstrapper, project resources, packaging
   scripts and workflow pins together. Run existing PE/signing, payload,
   provenance, portable/installer and normal Windows acceptance checks before
   publishing the matching VIIPER release and then the paired DS4Windows release.

## Compatibility scope to retain

The [USB identity compatibility notice](devices/genericjoystick.md#usb-identity-compatibility)
defines the stable USB and HID contract for `1209:D10C`.

The release notes should explain six axes, sixteen buttons and one POV, the two
axis layouts and four trigger modes. This device is for games accepting generic
joystick/DirectInput input; it does not add XInput, force feedback, audio or
adaptive-trigger support. Games may require manual axis/button bindings.

Changing from the preview identity/serial can require rebinding games. The
two-device synthetic Windows lab validated independent input and cleanup but
retained GUID changes when the first device was removed. Stable multi-controller
saved bindings remain unverified. Keep this limitation visible until a controlled
driver retest or another validated fix resolves it. Native validation of the
registered sixteen-button descriptor remains open.

See [device behavior and evidence](devices/genericjoystick.md) and the paired
DS4Windows release checklist for actual results and remaining acceptance work.
