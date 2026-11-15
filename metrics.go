// Env.Metrics: prometheus in server, kept behind this interface so it stays out of every extension
// (§4.5). Moved from sidecar's host_metrics.go (owner decision 251). A metric's name is urnetwork_<svc>_*.
// Its labels are route template, result, code, cluster, tier and fixed vocabularies of that kind (§19.8),
// never a network, repo, client or address, so a label value comes from a closed set, not a request
// (I-13).

package operator

// Declares metrics. Declaring a name again with the same kind and labels returns the same metric, so two
// engines in one process, or a restarted one in a test, can share a name. A second kind or label set for a
// name is the caller's bug, and the host panics, as prometheus's MustRegister does. Safe for concurrent
// use.
type Metrics interface {
	// A total that only rises.
	Counter(name string, help string, labelNames ...string) MetricsCounter
	// A level that moves both ways.
	Gauge(name string, help string, labelNames ...string) MetricsGauge
	// A distribution. bucketUpperBounds in seconds or bytes, as the metric's name says. nil takes the
	// host's default.
	Histogram(name string, help string, bucketUpperBounds []float64, labelNames ...string) MetricsHistogram
}

// A declared counter. labelValues follow labelNames in order, here and in the two below.
type MetricsCounter interface {
	// Takes a value of zero or more.
	Add(value float64, labelValues ...string)
}

// A declared gauge.
type MetricsGauge interface {
	// Replaces the level.
	Set(value float64, labelValues ...string)
}

// A declared histogram.
type MetricsHistogram interface {
	// Counts value into its bucket.
	Observe(value float64, labelValues ...string)
}
