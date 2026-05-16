package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// makeClusterID derives a short, stable ID from a cluster signature.
// Form: <prefix><N hex chars>. Length controlled by IDHexLen.
func makeClusterID(signature, prefix string, hexLen int) ClusterID {
	sum := sha256.Sum256([]byte(signature))
	h := hex.EncodeToString(sum[:])
	if hexLen > len(h) {
		hexLen = len(h)
	}
	return ClusterID(prefix + h[:hexLen])
}

// disambiguate appends a numeric suffix to id so that the returned
// value is unique within taken. taken is mutated to include the
// returned ID.
func disambiguate(id ClusterID, taken map[ClusterID]struct{}) ClusterID {
	if _, clash := taken[id]; !clash {
		taken[id] = struct{}{}
		return id
	}
	for n := 2; ; n++ {
		candidate := ClusterID(fmt.Sprintf("%s-%d", id, n))
		if _, clash := taken[candidate]; !clash {
			taken[candidate] = struct{}{}
			return candidate
		}
	}
}
