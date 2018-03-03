## `pkg/rootless-protobuf` ##

The protobuf for this package comes from the [`rootlesscontaine.rs`
project][rootlesscontaine.rs], which has all of the relevant information for
what this protobuf blob means. Effectively this is used in rootless mode for
the purposes of emulating `chown(2)` within a rootless container (using
something like `remainroot` or `PRoot`).

The protobuf itself comes from [the website][protobuf-source], and this
package's sources can be regenerated using `go generate` (assuming you have
`protoc` set up).

[rootlesscontaine.rs]: https://rootlesscontaine.rs/
[protobuf-source]: https://rootlesscontaine.rs/proto/rootlesscontainers.proto
