package coretest

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/urnetwork/connect"
	operator "github.com/urnetwork/operator-core"
)

// The conversion that is the whole boundary: the two layouts are one [16]byte.
var (
	_ = connect.Id(operator.Id{})
	_ = operator.Id(connect.Id{})
)

// operator.Id is server.Id moved, delegating to connect: its bytes, String and JSON are connect.Id's, for
// ids connect mints and for the edges of the byte range.
func TestIdReadsAsConnectIdDoes(t *testing.T) {
	connectIds := []connect.Id{{}, {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}}
	for i := 0; i < 1000; i += 1 {
		connectIds = append(connectIds, connect.NewId())
	}
	for _, connectId := range connectIds {
		id := operator.Id(connectId)
		if !bytes.Equal(id.Bytes(), connectId.Bytes()) {
			t.Fatalf("bytes %x, connect %x", id.Bytes(), connectId.Bytes())
		}
		if id.String() != connectId.String() {
			t.Fatalf("string %q, connect %q", id, connectId)
		}
		idJson, err := json.Marshal(id)
		if err != nil {
			t.Fatal(err)
		}
		connectJson, err := json.Marshal(connectId)
		if err != nil {
			t.Fatal(err)
		}
		if string(idJson) != string(connectJson) {
			t.Fatalf("json %s, connect %s", idJson, connectJson)
		}
		var decoded operator.Id
		if err := json.Unmarshal(connectJson, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != id {
			t.Fatalf("decoded connect's json as %v, want %v", decoded, id)
		}
		var connectDecoded connect.Id
		if err := json.Unmarshal(idJson, &connectDecoded); err != nil {
			t.Fatal(err)
		}
		if connectDecoded != connectId {
			t.Fatalf("connect decoded %v, want %v", connectDecoded, connectId)
		}
	}
}

func TestIdSortsAsConnectIdDoes(t *testing.T) {
	previous := connect.NewId()
	for i := 0; i < 1000; i += 1 {
		next := connect.NewId()
		for _, pair := range [][2]connect.Id{{previous, next}, {next, previous}, {next, next}} {
			a, b := pair[0], pair[1]
			if operator.Id(a).Cmp(operator.Id(b)) != a.Cmp(b) {
				t.Fatalf("Cmp of %v and %v disagrees", a, b)
			}
			if operator.Id(a).Less(operator.Id(b)) != a.LessThan(b) || operator.Id(a).LessThan(operator.Id(b)) != a.LessThan(b) {
				t.Fatalf("order of %v and %v disagrees", a, b)
			}
		}
		previous = next
	}
}
