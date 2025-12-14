package dashboard

type tailBuffer struct {
	max    int
	values []string
}

func newTailBuffer(max int) *tailBuffer {
	if max <= 0 {
		max = 1
	}
	return &tailBuffer{max: max}
}

func (t *tailBuffer) add(line string) {
	t.values = append(t.values, line)
	if len(t.values) > t.max {
		drop := len(t.values) - t.max
		t.values = t.values[drop:]
	}
}

func (t *tailBuffer) lines() []string {
	return append([]string(nil), t.values...)
}
