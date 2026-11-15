package operator

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/urnetwork/connect"
)

func TestNotGrantedErrorMatchesErrNotGranted(t *testing.T) {
	var err error = &NotGrantedError{Service: "Db", Reason: "evs holds no Postgres"}
	if !errors.Is(err, ErrNotGranted) {
		t.Errorf("%v does not match ErrNotGranted", err)
	}
	var notGranted *NotGrantedError
	if !errors.As(err, &notGranted) || notGranted.Service != "Db" {
		t.Errorf("errors.As gave %+v", notGranted)
	}
	if err.Error() != "Db: service not granted: evs holds no Postgres" {
		t.Errorf("error %q", err)
	}
	if text := (&NotGrantedError{Service: "Push"}).Error(); text != "Push: service not granted" {
		t.Errorf("error %q", text)
	}
}

func TestPrincipalValidateRefusesWhatNamesNoOne(t *testing.T) {
	zero := connect.Id{}
	valid := func() *Principal {
		return &Principal{NetworkId: connect.NewId(), ClientId: connect.NewId(), Audience: "evs"}
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, refused := range []*Principal{
		nil,
		func() *Principal { p := valid(); p.NetworkId = zero; return p }(),
		func() *Principal { p := valid(); p.ClientId = zero; return p }(),
		func() *Principal { p := valid(); p.Audience = ""; return p }(),
		func() *Principal { p := valid(); p.SidecarId = &zero; return p }(),
		func() *Principal { p := valid(); p.MemberId = &zero; return p }(),
		func() *Principal { p := valid(); p.SubjectClientId = &zero; return p }(),
	} {
		if err := refused.Validate(); !errors.Is(err, ErrAuthenticationRefused) {
			t.Errorf("%+v: %v", refused, err)
		}
	}
}

func TestAbuseStandingKnowsItsFive(t *testing.T) {
	for _, standing := range []AbuseStanding{AbuseStandingProbation, AbuseStandingStandard, AbuseStandingTrusted, AbuseStandingRestricted, AbuseStandingSuspended} {
		if !standing.Known() {
			t.Errorf("%s unknown", standing)
		}
	}
	for _, standing := range []AbuseStanding{"", "banned", "Standard"} {
		if standing.Known() {
			t.Errorf("%q known", standing)
		}
	}
}

func TestBlobPutOptionsValidateRefusesWhatTheStoreWould(t *testing.T) {
	var none *BlobPutOptions
	if err := none.Validate(); err != nil {
		t.Errorf("nil options: %v", err)
	}
	for _, refused := range []*BlobPutOptions{
		{Sha256: make([]byte, 31)},
		{Metadata: map[string]string{"": "a"}},
		{Metadata: map[string]string{"Upper": "a"}},
		{Metadata: map[string]string{"state": "a\x00"}},
		{Metadata: map[string]string{"state": " a"}},
		{Metadata: map[string]string{"state": strings.Repeat("a", 2*1024)}},
	} {
		if err := refused.Validate(); !errors.Is(err, ErrBlobPutOptions) {
			t.Errorf("%+v: %v", refused, err)
		}
	}
	if err := (&BlobPutOptions{Sha256: make([]byte, 32), Metadata: map[string]string{"state": "pending"}}).Validate(); err != nil {
		t.Errorf("valid options: %v", err)
	}
}

// The metadata limit is the settings', whose default is the store's 2 KiB, prefix included.
func TestBlobPutOptionsMetadataLimitIsTheSettings(t *testing.T) {
	prefixByteCount := len(blobMetadataKeyPrefix)
	if DefaultBlobPutSettings().MetadataMaxByteCount != 2*1024 {
		t.Errorf("default %d", DefaultBlobPutSettings().MetadataMaxByteCount)
	}
	options := &BlobPutOptions{Metadata: map[string]string{"k": strings.Repeat("a", 2*1024-prefixByteCount-1)}}
	if err := options.Validate(); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	smaller := &BlobPutSettings{MetadataMaxByteCount: prefixByteCount + 2}
	if err := options.ValidateWithSettings(smaller); !errors.Is(err, ErrBlobPutOptions) {
		t.Errorf("past a smaller limit: %v", err)
	}
	if err := (&BlobPutOptions{Metadata: map[string]string{"k": "a"}}).ValidateWithSettings(smaller); err != nil {
		t.Errorf("within a smaller limit: %v", err)
	}
}

func TestBlobUnknownErrorUnwrapsItsCause(t *testing.T) {
	cause := errors.New("eof")
	var err error = &BlobUnknownError{Err: cause}
	if !errors.Is(err, cause) {
		t.Errorf("%v does not unwrap to its cause", err)
	}
}

// sidecar's testdata/gates/table.yml pins these, as server's reader does.
func TestServiceGateBucketKeepsThePinnedBuckets(t *testing.T) {
	for idStr, bucket := range map[string]int{
		"5ca1ab1e-0000-4000-8000-000000000004": 0,
		"5ca1ab1e-0000-4000-8000-00000000009a": 1,
		"5ca1ab1e-0000-4000-8000-00000000007b": 49,
		"5ca1ab1e-0000-4000-8000-000000000028": 99,
	} {
		networkId, err := connect.ParseId(idStr)
		if err != nil {
			t.Fatal(err)
		}
		if got := ServiceGateBucket(networkId); got != bucket {
			t.Errorf("%s: bucket %d, want %d", idStr, got, bucket)
		}
	}
}

func TestServiceGateAdmitsListedAndBucketedNetworksOnlyWhileEnabled(t *testing.T) {
	listed := connect.NewId()
	lowBucket, _ := connect.ParseId("5ca1ab1e-0000-4000-8000-000000000004")
	highBucket, _ := connect.ParseId("5ca1ab1e-0000-4000-8000-000000000028")
	gate := NewServiceGate(true, true, []connect.Id{listed}, 50)
	if !gate.Admits(listed) || !gate.Admits(lowBucket) || gate.Admits(highBucket) {
		t.Errorf("enabled gate admitted wrongly")
	}
	if !gate.Writes() || gate.Percent() != 50 {
		t.Errorf("writes %v, percent %d", gate.Writes(), gate.Percent())
	}
	disabled := NewServiceGate(false, true, []connect.Id{listed}, 100)
	if disabled.Admits(listed) || disabled.Writes() {
		t.Errorf("disabled gate admitted or took writes")
	}
	if typo := NewServiceGate(true, true, nil, 101); typo.Percent() != 0 || typo.Admits(lowBucket) {
		t.Errorf("a percent past 100 opened %d", typo.Percent())
	}
	if (ServiceGate{}).Admits(listed) {
		t.Errorf("the zero gate admitted")
	}
}

func TestFormat2GateObservesUnlessExactlyOn(t *testing.T) {
	gate := NewFormat2Gate(NewServiceGate(true, true, nil, 100), "on", "On")
	if gate.Purge() != Format2On || gate.Drop() != Format2Observe {
		t.Errorf("purge %s, drop %s", gate.Purge(), gate.Drop())
	}
	if (Format2Gate{}).Purge() != Format2Observe || (Format2Gate{}).Drop() != Format2Observe {
		t.Errorf("the zero gate deletes")
	}
}

func TestWriteBudgetValuesKeepTheirVocabulary(t *testing.T) {
	if !WriteP0KindRevocation.Valid() || !WriteP0KindRotation.Valid() || WriteP0Kind("other").Valid() {
		t.Errorf("P0 kinds misread")
	}
	if !WriteP2RankEvacuation.Valid() || !WriteP2RankDrop.Valid() || WriteP2Rank(WriteP2RankCount).Valid() || WriteP2Rank(-1).Valid() {
		t.Errorf("P2 ranks misread")
	}
	if WriteP2RankPageOutFold.String() != "page_out_fold" || WriteP2Rank(WriteP2RankCount).String() != "invalid" {
		t.Errorf("rank names misread")
	}
	if err := (WriteCost{ImmediateByteCount: -1}).Validate(); !errors.Is(err, ErrWriteCost) {
		t.Errorf("a negative cost: %v", err)
	}
	if cost := (WriteCost{ImmediateByteCount: 3, DeferredByteCount: 4}); cost.ByteCount() != 7 || cost.Validate() != nil {
		t.Errorf("cost %+v", cost)
	}
	if (&WriteRefusal{RetryAfter: -time.Second}).RetryAfterMs() != 0 || (&WriteRefusal{RetryAfter: 1500 * time.Millisecond}).RetryAfterMs() != 1500 {
		t.Errorf("retry_after_ms misread")
	}
}

func TestDbBatchKeepsItsStatementsInOrder(t *testing.T) {
	var batch DbBatch
	batch.Queue("SELECT 1")
	batch.Queue("SELECT $1", 2)
	if batch.Len() != 2 {
		t.Fatalf("len %d", batch.Len())
	}
	sql, args := batch.Statement(1)
	if sql != "SELECT $1" || len(args) != 1 || args[0] != 2 {
		t.Errorf("statement %q %v", sql, args)
	}
}

// Each option kind is a DbOption, so a call takes it.
var _ = []DbOption{
	DbIsolationSerializable, DbReadWrite, DbRetryNone, DbLockTimeout(time.Second),
	DbReadBeforeBegin(nil),
}
