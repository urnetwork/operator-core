// Env.Clock and Env.ClockCheck, moved from sidecar's clock.go and host_clock.go (owner decision 251).
//
// Clock is the one clock an extension reads (§22.4, D-TEST-10, R-TEST-1): every reading of time and every
// wait goes through one, so a test moves time with a fake clock and never sleeps, and one host can run
// fast or slow beside the others. It names no connect type, since sidecar declares it on every leg.
//
// ClockCheck is how far this process's clock may stand from true time (§19.6's clocks row). §11.8's fence
// margin, §11.7's collection margins and §19.6's deadline and settle rules hold only while every clock
// that writes fence_ms, a refusal record's until or a deadline is within 50 ms of UTC, so a process asks
// before it writes one. While the answer is not sure, the work waits and nothing it would have done is
// lost.

package operator

import (
	"time"
)

// What a Sidecar, an engine or a host reads time from. Implementations are safe for concurrent use.
type Clock interface {
	// The time on this clock.
	Now() time.Time
	// Fires once this clock reaches now plus timeout. A timeout that is not positive fires at once.
	After(timeout time.Duration) <-chan time.Time
	// Calls function once this clock reaches now plus timeout, on a goroutine of the clock's choosing, so
	// function must not assume the caller's locks are held or free. stop returns false once function has
	// been called or stopped, and true when it stopped a call still to come.
	AfterFunc(timeout time.Duration, function func()) (stop func() bool)
}

// One process's view of its own clock. A check starts unsure and stays so until it has measured, so a
// process that cannot reach its time source fails closed. Safe for concurrent use, and no method does I/O,
// so each may be called on a request's path.
type ClockCheck interface {
	// The most this clock may be from true time, as a magnitude: the measured offset plus the time
	// source's root distance, which is RFC 5905 §4's maximum error. known is false while there is no such
	// bound, and offset is then zero. The host exports it, and monitor warns past 25 ms or while it is
	// unknown (§19.8).
	Offset() (offset time.Duration, known bool)
	// Whether Offset is known and within the bound §11.8's margins allow, 50 ms.
	Sure() bool
	// Closed the moment this clock stops being sure, and already closed while it is unsure, so a DELETE
	// started on an unsure clock cancels at once. Each sure stretch has its own channel. A caller takes
	// this before reading Sure, and takes it again for each piece of work, so a turn between the two reads
	// as unsure and never as sure.
	UnsureChannel() <-chan struct{}
}
