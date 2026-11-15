// Package operator is the host contract between a hosting process and the extensions it runs (DESIGN.md
// §4.4, §4.5; owner decision 251): the interfaces of the core services an extension's server-hosted code
// uses, rooted at Env, and the value types server and its extensions share. server imports it and
// implements it in server/core; sidecar imports it and takes it; it imports connect, and nothing it
// imports imports it back.
//
// It holds no service implementation and no global service state. Where server keeps package-level
// functions in one shared namespace (server.Db, server.Tx, its Redis and blob helpers), this package
// declares interfaces whose instances an Env creates, so a host can give each extension services of its
// own. The value types' own methods, and the constructors and pure helpers those methods need, are the
// only code here.
//
// Until the build's review phase moves the call sites (§4.18), sidecar and server keep Go type aliases of
// what moved here, and sidecar keeps ServiceHost, a struct of these interfaces, so every engine compiles
// unchanged and each ServiceHost.<member> names the Env accessor of that name.
package operator

import (
	"errors"
	"fmt"
)

// What a hosting process supplies: the root factory of every service an extension holds. Each accessor
// answers its service, or an error that wraps ErrNotGranted when this process may not hold the service or
// was not given it, never a nil service with a nil error. Implementations are safe for concurrent use, and
// an accessor does no I/O, so an extension may call one per request.
//
// Engines never import server, so go-redis, minio-go, pgx and prometheus stay out of every extension.
type Env interface {
	// The database, granted only where the process's role allows: evs and gkm get none, and keep their
	// durable state in Blobs alone (owner decisions 2, 19, 21).
	Db() (Db, error)
	// cas, incr, scripts, TTLs and hash-tagged keys. evs and gkm keep only soft state they rebuild here
	// (owner decision 136), and keyspace events come only through the shared subscriber, never per client.
	Redis() (Redis, error)
	// The segment bucket and each metadata cluster's bucket, by cluster id, the first holding place/
	// (owner decision 145): If-Match and If-None-Match puts, streamed put, ranged get, prefix list, delete
	// (X13).
	Blobs() (BlobStore, error)
	// jwt to Principal (server/session).
	Authenticator() (Authenticator, error)
	// Reserve, commit, release (§20).
	Quota() (Quota, error)
	// prometheus in server.
	Metrics() (Metrics, error)
	// Once-keyed jobs (taskworker).
	Tasks() (Tasks, error)
	// Server to client (X8).
	Push() (Push, error)
	// jwt and urls for an engine's connect client.
	ServiceClient() (ServiceClientSource, error)
	// The time the extension reads and waits on.
	Clock() (Clock, error)
	// How far this process's clock may be from true time (§19.6).
	ClockCheck() (ClockCheck, error)
	// §19.11's gates.
	Gates() (ServiceGates, error)
	// §11.13's write budget, one per process.
	Budget() (WriteBudget, error)
	// §21.7's standing and network_verified, from memory the host fills off the path.
	Abuse() (AbuseGate, error)
}

// What every refused Env accessor's error wraps.
var ErrNotGranted = errors.New("service not granted")

// An Env accessor's refusal, naming the service. errors.Is matches it to ErrNotGranted.
type NotGrantedError struct {
	// The accessor that refused, such as "Db".
	Service string
	// Why, for logs: the process's role refuses it, or the host was given none. Names no id or secret.
	Reason string
}

// The service and the reason.
func (self *NotGrantedError) Error() string {
	if self.Reason == "" {
		return fmt.Sprintf("%s: %v", self.Service, ErrNotGranted)
	}
	return fmt.Sprintf("%s: %v: %s", self.Service, ErrNotGranted, self.Reason)
}

// ErrNotGranted, so errors.Is finds it.
func (self *NotGrantedError) Unwrap() error {
	return ErrNotGranted
}
