// The object records of server's general blob store (server/blob.go), moved here as shared value types
// (owner decision 251). server.BlobObject is BlobListObject here, since BlobObject names an open Get of a
// file cluster (blobs.go); server keeps its own name through an alias. The store interfaces that return
// these stay in server.

package operator

import (
	"time"
)

// A stored object's key and size, as a list returns it.
type BlobListObject struct {
	Key  string
	Size int64
}

// A version is a separate retained storage charge, including a noncurrent version of the same key.
// Delete markers carry no object-body bytes.
type BlobObjectVersion struct {
	Key          string
	Size         int64
	VersionId    string
	DeleteMarker bool
}

// Immutable retention proof returned for a write that the backend protects through the requested absolute
// date. Production MinIO writes use compliance object lock and name the exact retained object version. The
// local backend is only a development substitute and reports mode LOCAL.
type BlobRetention struct {
	Key         string
	Size        int64
	VersionId   string
	Mode        string
	RetainUntil time.Time
}

// A bounded namespace capacity proof. CapacityBytes is an operator allocation for the configured prefix,
// not an estimate of the whole MinIO cluster; cluster free-space alerts remain sourced from MinIO metrics.
type BlobUsage struct {
	Authority              string  `json:"authority"`
	Bucket                 string  `json:"bucket"`
	Prefix                 string  `json:"prefix"`
	ObjectCount            int     `json:"object_count"`
	UsedBytes              int64   `json:"used_bytes"`
	CapacityBytes          int64   `json:"capacity_bytes"`
	FreeBytes              int64   `json:"free_bytes"`
	UsedPercent            float64 `json:"used_percent"`
	IncludesObjectVersions bool    `json:"includes_object_versions,omitempty"`
}

// The non-secret production proof for one immutable evidence bucket. ReplicationTargets contains only
// configured destination bucket identities; credentials and endpoint secrets are never included.
type BlobProtection struct {
	Authority          string   `json:"authority"`
	Bucket             string   `json:"bucket"`
	ObjectLock         string   `json:"object_lock"`
	Versioning         bool     `json:"versioning"`
	ReplicationTargets []string `json:"replication_targets"`
}

// Expires (deletes) objects whose key starts with KeyPrefix once they are older than TTL. This is how
// per-stream retention is set: each stats stream maps to a key-prefix rule.
type BlobLifecycleRule struct {
	KeyPrefix string
	TTL       time.Duration
}
