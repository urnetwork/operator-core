// Env.Quota (§20.7), implemented by server: limits, rates, edge leases and usage. Moved from sidecar's
// host_quota.go, with QuotaRefusal and QuotaLimits from its api_quota.go, whose JSON every leg decodes
// (owner decision 251).

package operator

import (
	"context"
	"time"

	"github.com/urnetwork/connect"
)

// A refusal maps to 402 with x402 terms when a purchase lifts the limit, 429 with Retry-After when a
// window resets (server/controller/x402_controller.go:849; server/router/handler_utils.go:604-609), else
// Error{RESOURCE_EXHAUSTED} (§6.5) or stream quota.
type QuotaRefusal struct {
	LimitKey     string `json:"limit_key"`
	Limit        int64  `json:"limit"`
	Used         int64  `json:"used"`
	RetryAfterMs uint32 `json:"retry_after_ms"`
	// A purchase or a higher tier lifts it.
	Upgrade bool `json:"upgrade"`
	// For limit_key screen_review alone: the review case, in its canonical uuid form, that the refusal opened or
	// joined, so the network can name it (QD-235, QD-1014). Empty, and left out, for every other limit.
	CaseId string `json:"case_id,omitempty"`
}

// A network's effective limits after its network_sidecar_limit rows (NetworkSidecarLimits, §20.5):
// Quota.Limits's result and GET /sidecar/limits's body. Producers cache it 5 s (§20.7).
type QuotaLimits struct {
	// "free" (Community), "pro" or "team" (§20.5).
	Tier string `json:"tier"`
	// By §20.4's keys, the same limit_key QuotaRefusal and network_sidecar_limit carry: its sidecar:
	// block's and its sidecar_abuse: block's, the directory_* keys among them, which §16.11's daemons and
	// readers apply (D-QUOTA-11). Bytes count as bytes, a true is 1 and a false 0, and -1 means none, as
	// `none` does in §20.4. A key absent here reads 0, which §20.5 defines as not included, so a limit this
	// build does not know fails closed.
	Limits map[string]int64 `json:"limits"`
	// enforce_sidecar_limits's state, which api sets from pro.yml's switch while Tier and Limits keep
	// reporting the plan as configured (§20.5's switch; owner decision 520). A local pre-check, a daemon's
	// or a reader's, refuses on a limit only while it is true, so its refusals ship dark as api's do; the
	// closure floor and pacing apply whatever it says, since they hold while a network is spent, which is
	// never dark. Absent reads false, so an older api's answer refuses nothing locally.
	Enforced bool `json:"enforced"`
}

// One producer's window of usage, sent every 60 s and flushed on SIGTERM. A producer retries an
// unanswered batch for at most 10 min, then drops it, counted and alerting: under-billing, never
// double-billing (§20.6).
type UsageBatch struct {
	// The retry identity: the first 16 B of SHA-256(producerInstanceId || windowSeq), where
	// producerInstanceId is 16 random bytes drawn at process start. A drop job's batch takes the first
	// 16 B of SHA-256(scanId || cursor range) instead (§20.6).
	BatchId connect.Id
	// "evs", "tlsedge", "mxedge", "smtpedge"; "taskworker" for evacuation's staged frees and the drop
	// job's deletes and holds.
	Producer string
	// 60 s windows.
	WindowStart time.Time
	Events      []*UsageEvent
}

// One network's usage on one meter in a batch. It names no repo, host or peer: §2.8's evs-9 lists
// transfer bytes per caller network and stored-byte deltas per repo network, and no more (I-18).
type UsageEvent struct {
	// Transfer: the caller's network. A stored-byte delta: the repo's network.
	NetworkId connect.Id
	// The meter it counts on, such as "tls" (§13.6).
	Meter string
	// Bytes to debit from the network's transfer balance.
	TransferByteCount int64
	// Moves the nightly stored count between passes (§20.7); negative when content is freed.
	StoredByteDelta int64
}

// Quota.OverLimit's answer, which evs and sockets re-read every 5 s, so writes overshoot by about a minute
// (§20.6). Each map holds the refusal to answer, keyed by network.
type QuotaOverLimit struct {
	// Over their storage limit: evs refuses their payload-adding uploads and pushes (§11.13).
	StorageNetworkIdRefusals map[connect.Id]*QuotaRefusal
	// Spent, with no balance left: new streams get quota and a Find names no ticket (§9.6), and evs
	// refuses as above (§20.6).
	SpentNetworkIdRefusals map[connect.Id]*QuotaRefusal
}

// §20.7. Producers cache limits 5 s. A zero network, debit or batch id is the caller's bug and an error,
// never a lookup: it names no one's limits or balance, and two calls sharing a zero id would read as one
// retried, the second counting nothing. Safe for concurrent use.
type Quota interface {
	// The network's effective limits.
	Limits(ctx context.Context, networkId connect.Id) (*QuotaLimits, error)
	// Counts keyAmounts against the network's rates in one all-or-none script on {sq_<network_id>}, once
	// per debitId, so a retry of a pass counts nothing twice. A non-nil refusal counted and recorded
	// nothing, so the same debitId asked again is judged afresh (DESIGN-DETAILS 13.2.1). Fails open for one
	// window if Redis is lost (§20.7).
	CountRate(ctx context.Context, networkId connect.Id, keyAmounts map[string]int64, debitId connect.Id) (*QuotaRefusal, error)
	// A lease for one edge process on one network and meter, drawing on escrow's per-balance reservations,
	// 64 MiB at a time and 16 MiB on Community (§20.6). A non-nil refusal holds no lease and marks the
	// network spent.
	Lease(ctx context.Context, networkId connect.Id, meter string) (QuotaLease, *QuotaRefusal, error)
	// Every 60 s per producer; idempotent per BatchId (§20.6).
	ReportUsage(ctx context.Context, batch *UsageBatch) error
	// evs and sockets, every 5 s: networks over their storage limit, or spent.
	OverLimit(ctx context.Context) (*QuotaOverLimit, error)
}

// One lease, safe for concurrent use. It renews itself at 75% used (§20.6).
type QuotaLease interface {
	// Atomic, with no I/O, so it runs inline on a stream. False: the lease has run out, and the network is
	// spent until a new Lease says otherwise.
	Spend(byteCount int64) bool
	// Settles and releases, returning the unused rest in reverse draw order.
	Close()
}
