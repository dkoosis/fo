// Strategy: extend BulletItem with optional *ClusterRender (per Task 0 orient).
package view

// expandSet is the parsed form of --expand flag values.
// Empty set + all=false → all clusters collapsed (default).
type expandSet struct {
	ids map[string]struct{}
	all bool
}

func newExpandSet(values []string) expandSet {
	e := expandSet{ids: map[string]struct{}{}}
	for _, v := range values {
		if v == "all" {
			e.all = true
			continue
		}
		e.ids[v] = struct{}{}
	}
	return e
}

func (e expandSet) wants(id string) bool {
	if e.all {
		return true
	}
	_, ok := e.ids[id]
	return ok
}

// NewExpandSet exposes expandSet construction to cmd/fo and external tests.
func NewExpandSet(values []string) expandSet { return newExpandSet(values) }
