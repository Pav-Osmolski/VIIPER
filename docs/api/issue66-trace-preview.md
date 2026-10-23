# Issue 66 diagnostic preview

This preview extends the exact 0.1.12 runtime source with response-write and
connection-close metadata. It does not change authentication, framing, retries,
socket options or shutdown behaviour. Thanks to
[@cns00](https://github.com/cns00) for the logs and recording supplied for
[DS4Windows issue 66](https://github.com/Pav-Osmolski/DS4Windows/issues/66).

With debug logging enabled, connections have a correlation ID and remote port,
a read-only snapshot of their Windows linger setting, response-write begin/end
records and connection-close begin/end records. Reply diagnostics classify known
operations rather than retaining arbitrary paths. They record accepted byte
counts and numeric socket errors, never credentials or message contents.
Completed socket writes and successful closes are not proof of peer delivery.
Streaming controller/audio I/O is not wrapped or logged by this instrumentation.

`cmd/viiper-api-probe` is an independent Go executable which only reads an
explicit existing key file and sends up to 20 authenticated pings to loopback.
It does not load VIIPER configuration, create a key, start a server, create a
virtual controller, change a driver or change a security setting. Its JSON
output contains only bounded result metadata and can be correlated by port.
DS4Windows verifies the matching helper hash before running it.

The coordinated DS4Windows preview provides a Log-page button that runs its
own authenticated checks followed by the independent probe. Source, binary
pins, provenance and corresponding-source archive must be shipped together.
This remains diagnostic work; it is not a confirmed fix for the reporter's
intermittent connection reset.
