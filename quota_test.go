package operator

import (
	"encoding/json"
	"strings"
	"testing"
)

// QuotaRefusal's JSON, which every leg decodes (owner decision 251): case_id is there for a screen review's
// refusal alone (QD-235, QD-1014), and every other refusal's body is what it was before the field.
func TestQuotaRefusalCarriesACaseOnlyWhenSet(t *testing.T) {
	// a synthetic id in the canonical uuid form
	caseId := "0195a6b8-1c2d-7e3f-8a9b-0c1d2e3f4a5b"
	for _, c := range []struct {
		refusal QuotaRefusal
		encoded string
	}{
		{
			refusal: QuotaRefusal{LimitKey: "screen_review", CaseId: caseId},
			encoded: `{"limit_key":"screen_review","limit":0,"used":0,"retry_after_ms":0,"upgrade":false,"case_id":"` + caseId + `"}`,
		},
		{
			refusal: QuotaRefusal{LimitKey: "screen_review"},
			encoded: `{"limit_key":"screen_review","limit":0,"used":0,"retry_after_ms":0,"upgrade":false}`,
		},
		{
			refusal: QuotaRefusal{LimitKey: "evs_drops_per_day", Limit: 20, Used: 20, RetryAfterMs: 1500},
			encoded: `{"limit_key":"evs_drops_per_day","limit":20,"used":20,"retry_after_ms":1500,"upgrade":false}`,
		},
		{
			refusal: QuotaRefusal{LimitKey: "sidecar_clients", Limit: 20, Used: 20, Upgrade: true},
			encoded: `{"limit_key":"sidecar_clients","limit":20,"used":20,"retry_after_ms":0,"upgrade":true}`,
		},
	} {
		encoded, err := json.Marshal(&c.refusal)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != c.encoded {
			t.Errorf("%+v encodes as %s, want %s", c.refusal, encoded, c.encoded)
		}
		if set := c.refusal.CaseId != ""; strings.Contains(string(encoded), `"case_id"`) != set {
			t.Errorf("%s names a case, want %t", encoded, set)
		}
		decoded := QuotaRefusal{}
		if err := json.Unmarshal([]byte(c.encoded), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != c.refusal {
			t.Errorf("%s decodes as %+v, want %+v", c.encoded, decoded, c.refusal)
		}
	}

	// a body from before the field, and one whose case_id is empty, decode with no case
	for _, body := range []string{
		`{"limit_key":"dns_entries","limit":5,"used":5,"retry_after_ms":0,"upgrade":true}`,
		`{"limit_key":"dns_entries","limit":5,"used":5,"retry_after_ms":0,"upgrade":true,"case_id":""}`,
	} {
		decoded := QuotaRefusal{}
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatal(err)
		}
		if want := (QuotaRefusal{LimitKey: "dns_entries", Limit: 5, Used: 5, Upgrade: true}); decoded != want {
			t.Errorf("%s decodes as %+v, want %+v", body, decoded, want)
		}
	}
}
