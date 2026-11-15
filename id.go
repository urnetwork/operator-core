// The identifier server and its extensions share: server.Id moved here (owner decision 251). It is
// connect.Id's layout, a ULID, and its parsing, formatting and comparison delegate to connect, so an id
// reads, sorts and encodes alike on both sides of every boundary. server.Id and sidecar.Id alias it, and
// each keeps its own generator: server's NewId over connect.NewId, and sidecar's on crypto/rand.
//
// The database encoding is server's: the database/sql Scanner and Valuer here, and the pgx codec, which
// stays in server, registered for this type.

package operator

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/urnetwork/connect"
)

// A 48-bit big-endian millisecond timestamp, then 80 bits of entropy, so ids sort by creation time. The
// timestamp is the clock of whatever minted the id: a dependable order within one source and no global
// clock across sources. Comparable, so it keys maps.
type Id connect.Id

// What every refusal of a malformed id matches.
var ErrInvalidId = errors.New("invalid id")

// A refusal that keeps server.Id's words where server refused, and matches ErrInvalidId.
type invalidIdError struct {
	message string
}

// server.Id's words for the refusal.
func (self *invalidIdError) Error() string {
	return self.message
}

// ErrInvalidId, so errors.Is finds it.
func (self *invalidIdError) Unwrap() error {
	return ErrInvalidId
}

// A copy, since the receiver is one, so a write to it leaves the id alone.
func (self Id) Bytes() []byte {
	return self[0:16]
}

// Whether this id sorts before b, by creation time within one source: server.Id's name.
func (self Id) Less(b Id) bool {
	return connect.Id(self).LessThan(connect.Id(b))
}

// Less, by sidecar.Id's name, so callers of both compile.
func (self Id) LessThan(b Id) bool {
	return connect.Id(self).LessThan(connect.Id(b))
}

// -1, 0 or 1, by byte.
func (self Id) Cmp(b Id) int {
	return connect.Id(self).Cmp(connect.Id(b))
}

// Lowercase hex in the 8-4-4-4-12 form, connect's and the database's uuid.
func (self Id) String() string {
	return connect.Id(self).String()
}

// The database/sql Scanner: a uuid's text. A nil source is refused, so a nullable column scans into *Id.
func (self *Id) Scan(src any) error {
	if src == nil {
		return fmt.Errorf("Scan with nil source not supported by Id (use *Id)")
	}
	switch src := src.(type) {
	case string:
		id, err := connect.ParseId(src)
		if err != nil {
			return err
		}
		*self = Id(id)
		return nil
	}
	return fmt.Errorf("cannot scan %T", src)
}

// The database/sql/driver Valuer: the uuid's text.
func (self Id) Value() (driver.Value, error) {
	return connect.Id(self).String(), nil
}

// A value receiver, so an Id field has one encoding whether its struct is marshaled by value or by
// pointer: a pointer receiver leaves a non-addressable Id to encoding/json's byte array form.
func (self Id) MarshalJSON() ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteByte('"')
	buff.WriteString(connect.Id(self).String())
	buff.WriteByte('"')
	return buff.Bytes(), nil
}

// Takes a quoted id in the dashed form and refuses everything else, JSON null included. This is the
// union of what server.Id and sidecar.Id refused before they became this type: server's null and length
// refusals keep its words, and sidecar's own refusals, an unquoted value and a separator other than a dash
// that connect would drop unread, answer ErrInvalidId. Every refusal matches ErrInvalidId, and none repeats
// its input, which may not be an id at all (I-13). A field absent from the JSON never reaches this, so it
// keeps its value, the zero id in a fresh struct: a reader of a required id compares it to Id{}.
func (self *Id) UnmarshalJSON(src []byte) error {
	if bytes.Equal(src, []byte("null")) {
		return &invalidIdError{message: "Unmarshal with nil source not supported by Id (use *Id)"}
	}
	if len(src) != 38 {
		return &invalidIdError{message: fmt.Sprintf("invalid length for UUID: %v", len(src))}
	}
	if src[0] != '"' || src[37] != '"' {
		return ErrInvalidId
	}
	idStr := string(src[1:37])
	if idStr[8] != '-' || idStr[13] != '-' || idStr[18] != '-' || idStr[23] != '-' {
		return ErrInvalidId
	}
	id, err := connect.ParseId(idStr)
	if err != nil {
		// connect's error names the byte it refused, which is input
		return ErrInvalidId
	}
	*self = Id(id)
	return nil
}

// FNV-1's 64-bit hash of the 16 bytes, as server.Id's.
func (self Id) Hash() uint64 {
	h := fnv.New64()
	h.Write(self[0:16])
	return h.Sum64()
}
