package scene

import "time"

// Frame is one beat of cast playback: how long to wait before showing
// Content, then the screen contents to show. Content is a full render
// (full-redraw model) — the scene revealed so far, not a delta. Full
// redraw keeps the producer simple and every terminal handles it;
// deltas are a later optimization (see docs/design/cast-rail-visual.md).
type Frame struct {
	Delay   time.Duration
	Content string
}

// Cast walks the scene beat by beat, producing one Frame per beat. Each
// frame's Content is render(prefix) where prefix is the scene revealed up
// to and including that beat — so playback reveals the walkthrough
// progressively, the way a viewer would watch it unfold.
//
// render converts a (partial) Scene to its screen form — pass
// view.RenderSceneHumanString or RenderSceneLLMString. delay assigns the
// wait before each beat; a nil delay means every frame plays immediately
// (Delay 0). Cast does not import the view layer; the renderer is
// injected to avoid a cycle.
func Cast(s Scene, render func(Scene) string, delay func(Beat) time.Duration) []Frame {
	var frames []Frame
	for ai, act := range s.Acts {
		for bi, beat := range act.Beats {
			var d time.Duration
			if delay != nil {
				d = delay(beat)
			}
			frames = append(frames, Frame{
				Delay:   d,
				Content: render(scenePrefix(s, ai, bi)),
			})
		}
	}
	return frames
}

// scenePrefix returns s revealed up to and including act ai, beat bi:
// every act before ai in full, plus act ai truncated to its first bi+1
// beats. Title and Actors are preserved. The input is not mutated —
// only the truncated act's Beats slice is shared (read-only).
func scenePrefix(s Scene, ai, bi int) Scene {
	out := Scene{Title: s.Title, Actors: s.Actors}
	for i := 0; i <= ai; i++ {
		act := s.Acts[i]
		if i == ai {
			act.Beats = act.Beats[:bi+1]
		}
		out.Acts = append(out.Acts, act)
	}
	return out
}
