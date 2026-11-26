# Nintendo GameCube Controller for Switch 2

Use `ns2gamecube` to create the emulated USB controller. It uses Nintendo VID:PID
`057E:2073` and a distinct USB device revision so DS4Windows can tell the
virtual controller apart from physical hardware.

The 24-byte client input frame starts with the same little-endian button mask
and four unsigned 12-bit stick coordinates as `ns2pro`. The L and R trigger
travel bytes are at offsets 12 and 13, with a nominal full-travel value of 232.
Bytes 14 through 23 are reserved. The ZL and ZR mask bits represent the two
independent trigger clicks. Stick center is 2048 and positive Y points up.

Feedback frames are 34 bytes. Flag `0x04` at byte 32 signals single-motor
rumble. Byte 0 is 255 for motor on and 0 for off. Flag `0x02` identifies the
player LED mask in byte 33. See the [Switch 2 Pro documentation](ns2pro.md) for
the shared button mask and bulk command transport.
