// Env.Abuse: a network's §21.7 standing and its network_verified, as §8's contract gate and §12-§15's
// gates read them (D-ABUSE-4). Moved from sidecar's host_abuse.go, and AbuseStanding from its api_abuse.go
// (owner decision 251). api keeps both in Postgres, and an inline packet or frame path never queries
// Postgres (I-12), so a gate reads them from memory the host fills off the path.
//
// Every answer fails closed. ok false means the host has not loaded the network, and a gate refuses on it,
// never passes: a miss is the host's cue to load the network off the path, never the gate's to wait or to
// ask Postgres. A standing this build does not know refuses too (AbuseStanding.Known). These facts only
// take away: a server originates only denials (I-4), so a gate passes on them nothing that a network's
// signed grants do not already allow.

package operator

import (
	"github.com/urnetwork/connect"
)

// A network's §21.7 standing, as Env.Abuse and GET /abuse/status give it. The values are a format:
// server's model.AbuseStanding holds the same strings.
type AbuseStanding string

const (
	// Under 7 days old: same-network mail deposits only, 1 endpoint publishing at most 24 h, at most
	// probation_dns_entries user RRsets and none A, AAAA or CNAME, and no names held for review, aliases
	// or outbound shares.
	AbuseStandingProbation AbuseStanding = "probation"
	// 7 or more days old, and 30 days without an action: the tier's defaults (§20).
	AbuseStandingStandard AbuseStanding = "standard"
	// A Team with a verified domain, paid 30 days and clean 90: Team thresholds, and brand use.
	AbuseStandingTrusted AbuseStanding = "trusted"
	// A network halt or a kill within 30 days: probation's limits.
	AbuseStandingRestricted AbuseStanding = "restricted"
	// A §8 suspension until reviewed: no reach, publishing or sending, while its data stays kept, readable
	// and exportable.
	AbuseStandingSuspended AbuseStanding = "suspended"
)

// Whether this build knows the standing. A gate refuses one it does not, as it refuses a network it has
// not loaded, so a standing added later never passes an older gate.
func (self AbuseStanding) Known() bool {
	switch self {
	case AbuseStandingProbation, AbuseStandingStandard, AbuseStandingTrusted, AbuseStandingRestricted, AbuseStandingSuspended:
		return true
	}
	return false
}

// A network's standing and verification, from memory. A method does no I/O and never blocks, so it may be
// called on an inline packet or frame path. Safe for concurrent use.
type AbuseGate interface {
	// The network's §21.7 standing. ok is false while the network is not loaded.
	Standing(networkId connect.Id) (standing AbuseStanding, ok bool)
	// Whether the network is verified (§21.7): a payment method, or its owner's verified phone or email.
	// Public names and internet mail need it. ok is false while the network is not loaded, which a gate
	// refuses as it refuses an unverified network.
	Verified(networkId connect.Id) (verified bool, ok bool)
}
