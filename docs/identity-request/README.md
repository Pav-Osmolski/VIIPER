# pid.codes request — submitted, allocation pending

Prepared and submitted on 2026-10-15 for the twelve-button generic joystick in
this fork: [pid.codes PR #1317](https://github.com/pidcodes/pidcodes.github.com/pull/1317).
**1209:D10C is a request candidate, not an allocated identity.** Do not put it in
the descriptor or remove the DS4Windows portable-lab restriction before approval.

The two `index.md` files below were submitted through a fork of
[pidcodes/pidcodes.github.com](https://github.com/pidcodes/pidcodes.github.com):

- `org/Pav-Osmolski/index.md`
- `1209/D10C/index.md`

The proposed owner is the maintainer of this fork, not a claim to represent
upstream VIIPER contributors. The requested license matches this fork's server
and device implementation. Generated client libraries have separate notices.

## Submitted request text

Title: **Request 1209:D10C for VIIPER Generic Joystick (12 buttons)**

Please allocate 1209:D10C to the software-only generic joystick implemented in
my VIIPER fork and fed by my DS4Windows fork.

The device is a USB/IP virtual HID joystick with six absolute axes, twelve
buttons and one POV hat. Its purpose is to give DirectInput/WinMM games ordinary
joystick axes with configurable trigger resting positions, rather than requiring
a separate mapper application. It has no physical hardware, force feedback or
audio interfaces. The complete descriptor and report implementation are public
GPL-3.0-or-later Go source, linked in the registry entry.

One fixed identity is needed because Windows and games retain device information
and controller bindings. Asking each user to obtain their own identity would
make installation, troubleshooting and shared binding configurations inconsistent.
The source includes an optional sixteen-button experimental descriptor, but this
request is only for the twelve-button descriptor tested with DS4Windows. We will
not use this PID for the sixteen-button descriptor or incompatible future layouts.

The prototype currently uses explicitly unallocated lab identities and is gated
to an isolated DS4Windows preview. We will only adopt the requested identity
following acceptance. Native x86/x64 DirectInput and WinMM tests, USB DualSense
lifecycle testing, and user gameplay tests in BeamNG.drive and the current Steam
FlatOut 2 installation have passed. These are limited compatibility results,
not claims about all legacy games.

## Review and submission procedure

1. Obtain the fork maintainer's approval to submit this external request.
2. Recheck the registry and open pull requests immediately before submission;
   neither an existing `1209/D10C` entry nor a PR search match was found during
   preparation. This is not a reservation.
3. Copy only the two registry entries into a dedicated branch of a registry fork,
   and submit the title/body above. Do not include this preparation README.
4. Wait for acceptance and confirm the published registry entry. Keep production
   activation disabled while the request is pending or if it is declined.
5. Record the accepted identity and request URL in the release checklist, then
   update the descriptor and test the resulting new Windows device identity.

## Basis for the request

- [Application rules](https://pid.codes/howto/) allow software projects to apply,
  with justification for needing a shared identity. Acceptance is discretionary.
- [Gabbro's software-only HID allocation](https://github.com/pidcodes/pidcodes.github.com/blob/master/1209/6ABB/index.md)
  provides a software-only virtual-device precedent.
- [usbip-win Virtual HUB](https://pid.codes/1209/8250/) provides a USB/IP precedent;
  its PID belongs to that device and must not be reused here.
- [1209:0001](https://pid.codes/1209/0001/) is for private testing and must not be
  redistributed as the release identity.

The experimental `FFFF:D110` PID does not imply `1209:D110` is available:
that registry PID is already allocated to another project. The sixteen-button
variant remains a separate lab-only investigation.
