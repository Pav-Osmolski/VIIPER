# Management reply regressions

Generate the actual clients first:

```text
go run ./cmd/viiper codegen
```

Run the Go transport tests with `go test ./viiperclient`. For generated clients:

```text
dotnet run --project _testing/management-replies/csharp/Replies.csproj -c Release
cargo test --manifest-path _testing/management-replies/rust/Cargo.toml
cd clients/typescript
npm install
npm run build
cd ../..
node _testing/management-replies/typescript.cjs clients/typescript
```

The tests use only synthetic streams or ephemeral loopback sockets. They do not
start the VIIPER server, attach a USB/IP device, read a deployment key or use a
physical controller. C# reflection targets the generated reply reader rather
than a copied implementation. Rust exercises the public synchronous and async
clients against a TCP peer held open after its reply. TypeScript exercises the
generated request method with deterministic socket/wrapper events, including
split UTF-8, late resets, incomplete responses and wrapper errors.

Keep complete LF-terminated replies, legacy clean-EOF replies, and pretty-printed
JSON working. Do not turn a reset before completion into a successful response
just because a prefix arrived. API errors and malformed JSON remain failures.
Authenticated record validation is separately covered by `_testing/authv2` and
the Go authentication tests; a partial ciphertext must never become plaintext.
