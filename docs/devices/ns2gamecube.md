# Nintendo GameCube Controller for Switch 2

Use `ns2gamecube` to create the emulated USB controller. It uses Nintendo VID:PID
`057E:2073` and a distinct USB device revision so DS4Windows can tell the
virtual controller apart from physical hardware.

The 26-byte client input frame starts with the same little-endian button mask
and four unsigned 12-bit stick coordinates as `ns2pro`. Bytes 12 to 23 carry
signed little-endian accelerometer X/Y/Z (4096 counts/g) and gyroscope X/Y/Z
(16.4 counts per degree/s). Six zeroes represent absent motion. The L and R
trigger travel bytes are at offsets 24 and 25, with a nominal full-travel
value of 232. The ZL and ZR mask bits represent the independent trigger
clicks. Stick center is 2048 and positive Y points up.

When the host enables the IMU feature, common report `0x05` carries the
motion sample directly. Native report `0x0A` carries the normal 40-byte
motion payload, with interpolated history and orientation derived from the
source gyroscope. See [chyyran's report analysis](https://gist.github.com/chyyran/76fdc5c2a3f20802266af913d44ef7b3)
for the original motion layout.

Feedback frames are 34 bytes. Flag `0x04` at byte 32 signals single-motor
rumble. Byte 0 is 255 for motor on and 0 for off. Flag `0x02` identifies the
player LED mask in byte 33. See the [Switch 2 Pro documentation](ns2pro.md) for
the shared button mask and bulk command transport.
