package operator

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"strings"
	"testing"

	"github.com/urnetwork/connect"
)

func TestIdStringIsDashedLowercaseHex(t *testing.T) {
	id := Id{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	if s := id.String(); s != "01234567-89ab-cdef-fedc-ba9876543210" {
		t.Errorf("string %q", s)
	}
}

func TestIdBytesIsACopy(t *testing.T) {
	id := Id{1}
	id.Bytes()[0] = 2
	if id != (Id{1}) {
		t.Errorf("writing through Bytes changed the id: %v", id)
	}
}

// Less is server.Id's name and LessThan sidecar.Id's: both callers compile, and both read one order.
func TestIdLessAndLessThanOrderByByte(t *testing.T) {
	low := Id{0x00, 0xff}
	high := Id{0x01, 0x00}
	if !low.Less(high) || high.Less(low) || low.Less(low) {
		t.Errorf("Less misordered %v and %v", low, high)
	}
	if !low.LessThan(high) || high.LessThan(low) || low.LessThan(low) {
		t.Errorf("LessThan misordered %v and %v", low, high)
	}
	if low.Cmp(high) != -1 || high.Cmp(low) != 1 || low.Cmp(low) != 0 {
		t.Errorf("Cmp misordered %v and %v", low, high)
	}
}

func TestIdJsonIsOneEncodingByValueAndPointer(t *testing.T) {
	type holder struct {
		Id    Id  `json:"id"`
		IdPtr *Id `json:"id_ptr"`
	}
	id := Id(connect.NewId())
	want := `{"id":"` + id.String() + `","id_ptr":"` + id.String() + `"}`
	byValue, err := json.Marshal(holder{Id: id, IdPtr: &id})
	if err != nil {
		t.Fatal(err)
	}
	byPointer, err := json.Marshal(&holder{Id: id, IdPtr: &id})
	if err != nil {
		t.Fatal(err)
	}
	if string(byValue) != want || string(byPointer) != want {
		t.Errorf("by value %s, by pointer %s, want %s", byValue, byPointer, want)
	}
	var decoded holder
	if err := json.Unmarshal(byValue, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Id != id || decoded.IdPtr == nil || *decoded.IdPtr != id {
		t.Errorf("decoded %+v", decoded)
	}
}

func TestIdJsonTakesUppercaseHex(t *testing.T) {
	id := Id(connect.NewId())
	var decoded Id
	if err := json.Unmarshal([]byte(`"`+strings.ToUpper(id.String())+`"`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != id {
		t.Errorf("decoded %v, want %v", decoded, id)
	}
}

// server.Id's refusals keep its words; every refusal matches ErrInvalidId and leaves the id alone.
func TestIdJsonRefusesServerIdsRefusalsInItsWords(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string
	}{
		{src: `null`, want: "Unmarshal with nil source not supported by Id (use *Id)"},
		{src: `""`, want: "invalid length for UUID: 2"},
		{src: `"not-an-id"`, want: "invalid length for UUID: 11"},
	} {
		id := Id{7}
		err := id.UnmarshalJSON([]byte(c.src))
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: %v, want %q", c.src, err, c.want)
		}
		if !errors.Is(err, ErrInvalidId) {
			t.Errorf("%s: %v does not match ErrInvalidId", c.src, err)
		}
		if id != (Id{7}) {
			t.Errorf("%s wrote %v", c.src, id)
		}
	}
}

// sidecar.Id's refusals, which server.Id let through: an unquoted value, a separator connect drops
// unread, and a byte hex refuses, none of which the error repeats (I-13).
func TestIdJsonRefusesSidecarIdsRefusalsWithoutEchoingThem(t *testing.T) {
	for _, src := range []string{
		`x01234567-89ab-cdef-fedc-ba9876543210x`,
		`"01234567-89ab-cdef-fedc-ba9876543210x`,
		`x01234567-89ab-cdef-fedc-ba9876543210"`,
		`"01234567x89ab-cdef-fedc-ba9876543210"`,
		`"01234567-89abxcdef-fedc-ba9876543210"`,
		`"01234567-89ab-cdefxfedc-ba9876543210"`,
		`"01234567-89ab-cdef-fedcxba9876543210"`,
		`"g1234567-89ab-cdef-fedc-ba9876543210"`,
		// 38 digits: valid JSON, a number, which server.Id once read as hex
		`12345678901234567890123456789012345678`,
	} {
		id := Id{7}
		err := id.UnmarshalJSON([]byte(src))
		if !errors.Is(err, ErrInvalidId) {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if strings.Contains(err.Error(), "g") || strings.Contains(err.Error(), "x") {
			t.Errorf("%s: error %q repeats its input", src, err)
		}
		if id != (Id{7}) {
			t.Errorf("%s wrote %v", src, id)
		}
	}
}

func TestIdJsonLeavesAnAbsentFieldZero(t *testing.T) {
	var decoded struct {
		Id Id `json:"id"`
	}
	if err := json.Unmarshal([]byte(`{}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Id != (Id{}) {
		t.Errorf("id %v", decoded.Id)
	}
}

func TestIdSqlValueAndScanRoundTrip(t *testing.T) {
	id := Id(connect.NewId())
	value, err := id.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != id.String() {
		t.Errorf("value %v, want %q", value, id.String())
	}
	var scanned Id
	if err := scanned.Scan(value); err != nil {
		t.Fatal(err)
	}
	if scanned != id {
		t.Errorf("scanned %v, want %v", scanned, id)
	}
}

func TestIdScanRefusesNilAndOtherTypes(t *testing.T) {
	for _, src := range []any{nil, 7, []byte("01234567-89ab-cdef-fedc-ba9876543210")} {
		id := Id{7}
		if err := id.Scan(src); err == nil {
			t.Errorf("%v scanned", src)
		}
		if id != (Id{7}) {
			t.Errorf("%v wrote %v", src, id)
		}
	}
}

func TestIdHashIsFnv1OfTheBytes(t *testing.T) {
	id := Id(connect.NewId())
	h := fnv.New64()
	h.Write(id[:])
	if id.Hash() != h.Sum64() {
		t.Errorf("hash %x, want %x", id.Hash(), h.Sum64())
	}
}
