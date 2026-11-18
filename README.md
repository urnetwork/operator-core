# operator-core

`github.com/urnetwork/operator-core`, Go package `operator`: the host contract between a URnetwork
server process and the extensions it hosts, such as sidecar's engines (owner decision 251; sidecar's
DESIGN.md §4.4, §4.5 and §4.7).

- `Env` is the root factory of every core service an extension holds: the database, Redis, the file
  clusters, the authenticator, quota, metrics, tasks, push, service clients, the clock and its check,
  the gates, the write budget and the abuse gate. Each accessor answers a service or an error that
  wraps `ErrNotGranted`, never a nil.
- The engine contract (`Engine`, `HttpEngine`, `StreamEngine`) and `Extension`, an extension's
  migrations and its engines' constructors over an `Env`, with the registry a host iterates.
- The value types server and its extensions share: `Id`, `ByteCount`, `NanoCents`, the blob object
  records and the sentinel errors the interfaces return.
- The global `Env`: `SetGlobalEnv`, which a hosting process calls once at its start, and `GlobalEnv`,
  which code a host calls with no `Env` in hand reads. Until one is set it answers an `Env` that
  refuses every service, never a nil.

This module holds no service implementation: server implements the interfaces in `server/core`. The
global `Env` is its one piece of process-wide state. It imports connect, and nothing it imports imports
it back. `coretest/` is a separate module of conformance suites and test helpers, which only test legs
import.

Development uses sibling checkouts through `replace` lines, as connect's other consumers do. Commits
pass `.githooks/commit-msg` (`git config core.hooksPath .githooks`).

Licensed under the Mozilla Public License 2.0 (`LICENSE`).
