# Virtual battery status v1

`bus/{busId}/{devId}/battery-status-v1` accepts one complete JSON object:

```json
{"version":1,"percent":50,"charging":false,"externalPower":false,"complete":false}
```

The route is supported by DualShock 4, DualSense/Edge and NS2 Pro. It changes
only mutable power metadata and preserves serial identity, factory date and
independent voltage readings. It does not change any input-stream packet size.
Sony reports quantize to ten-percent bands. NS2 Pro uses its existing 0–9 scale.
Full/complete is distinct from a battery that is still charging at 100%.

All five fields are required, including false and zero. Unknown or duplicate
fields, null, trailing JSON, unsupported versions, percentages above 100 and
inconsistent charge states are rejected before mutation. Payloads are limited
to 512 bytes. Successful replies contain `{"version":1,"updated":true}`.

The caller owns source selection and must supply reliable readings only. The
native DualSense physical-metadata stream remains authoritative when present.
Native NS2 clients should retain `ns2pro-status-v1` to propagate measured voltage.
Wired Xbox personas intentionally do not implement this Sony/Nintendo route.

Factory build dates describe the emulated firmware, not the hosting program.
