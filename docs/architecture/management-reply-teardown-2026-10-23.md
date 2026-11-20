# Bounded management-reply teardown

The final diagnostic preview for [DS4Windows issue 66](https://github.com/Pav-Osmolski/DS4Windows/issues/66)
compared immediate and graceful closing in immediate/graceful/graceful/immediate
order, on the same isolated port, using DS4Windows and an independent Go client.
Thanks to [@cns00](https://github.com/cns00) for five captures on the affected host.
The isolated server did not create controllers or attach USB devices.

Across the five captures, immediate closing failed 77 of 400 requests; graceful
closing failed 3 of 400. Separate live-server checks failed 5 of 400. Successful
controller creation after a bounded existing retry coexisted with residual resets.
These are single-host observations, not a guarantee of complete resolution or
evidence identifying a hardware defect, Windows version or security product.

On 24 October 2026 the reporter confirmed that the final paired DS4Windows
5.0.47 connection candidate consistently reached Ready, and the maintainer
authorized stable publication. The accepted implementation ships in VIIPER
0.1.14 with DS4Windows 5.0.47. This acceptance resolves the reported behaviour;
it does not identify a defective laptop component or guarantee all future resets.

The production implementation applies graceful teardown only after a complete
one-shot management reply has been written successfully. It shuts down TCP sends,
then drains at most 4097 bytes for at most 250 ms before the existing final close.
The raw socket stays registered during the drain so server shutdown interrupts
it. The drain never parses additional requests or changes encrypted framing.
Failed authentication, failed/short replies and device/audio streams retain
their existing immediate cleanup. There is no comparison mode or diagnostic
ping-only bypass in the production server.

Regression checks exercise reply delivery and FIN, silent and excessive peers,
server shutdown during drain, repeated authenticated replies and API errors,
connection ownership and the existing stream/import/retained-device lifecycle.
No additional state-changing operation retry is introduced. A successful write
still does not prove peer receipt; ambiguous acknowledgements remain errors.
