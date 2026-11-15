// The engine contract: what each engine a host runs hands back (§4.5), moved from sidecar's
// host_service.go (owner decision 251), so a host runs any extension's engines alike. The host wraps an
// engine with readiness, /status, stats and PROXY-unwrapped listeners and serves the shape it is handed,
// never asking by type assertion; pushservice's types run inside existing services instead.
//
// §4.5 declares one interface whose first member is Handler() or Serve. No Go interface says "one or the
// other", and smtpedge dials out and listens on nothing (§14.9), so it has neither: an engine serves
// through HttpEngine, StreamEngine or both, and one that listens on nothing is an Engine alone. What that
// costs is that a Handler of the wrong signature still compiles as an Engine, so each engine package says
// what it is, as var _ operator.HttpEngine = (*EvsEngine)(nil).

package operator

import (
	"net"
	"net/http"
)

// An engine's lifecycle. Safe for concurrent use.
type Engine interface {
	// Whether the stores this engine needs are reachable and its state is loaded, the column §19.4 gives
	// each service. The host gates its /status on this.
	Ready() bool
	// Takes no new work and returns once work in flight has finished or the engine's own bound has passed.
	// New connections are the host's to stop: it closes the listeners.
	Drain()
	// Ends everything at once and releases what the engine holds. Safe after Drain, and more than once.
	Close()
}

// An engine served over HTTP.
type HttpEngine interface {
	Engine
	// One handler for every route of the service, built once. The host mounts it behind readiness and
	// §19.8's route metrics.
	Handler() http.Handler
}

// An engine that serves raw connections, as tlsedge and mxedge do.
type StreamEngine interface {
	Engine
	// Accepts on listener until it closes or the engine closes, and then returns why it stopped, nil when
	// the engine closed. The host's listeners have already taken any PROXY header off. The host calls this
	// once for each listener, each on its own goroutine, as tlsedge's four ports need.
	Serve(listener net.Listener) error
}
