// The counting units server and its extensions share, moved from server/model (owner decision 251). Each
// stays an alias of int64, as server/model declared it, so arithmetic, encodings and every existing call
// site read exactly as before.

package operator

// A count of bytes.
type ByteCount = int64

// An amount of money as server counts it: 10^9 to the US dollar (server/model.UsdToNanoCents).
type NanoCents = int64
