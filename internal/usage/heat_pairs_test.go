package usage

import (
	"testing"
	"time"
)

// The heatmap's year and the whole history's page pair the same chunks
// once between them: on a cold start each paired every row of them, the
// heatmap's the third pairing of all of them and the last of the
// footprint's peaks (#2278). Only the heatmap's recent days are paired
// again for it.
func TestHeatmapPairsAsAllDoes(t *testing.T) {
	for _, heatFirst := range []bool{false, true} {
		name := "page first"
		if heatFirst {
			name = "heatmap first"
		}
		t.Run(name, func(t *testing.T) {
			pageHome(t)
			settleLogClock(t)
			holdClock(t, time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local))
			historyLog(t, 400)
			n := 0
			pairing = func() { n++ }
			t.Cleanup(func() { pairing = nil })
			var h Heatmap
			if heatFirst {
				h = HeatmapOf(Filter{})
				if n != 2 {
					t.Fatalf("the heatmap paired %d times, want 2: every row and its recent days", n)
				}
				QueryPage(All, Filter{}, 0, 1)
				if n != 2 {
					t.Fatalf("the page of All paired %d times after the heatmap, want 0", n-2)
				}
			} else {
				QueryPage(All, Filter{}, 0, 1)
				if n != 1 {
					t.Fatalf("the page of All paired %d times, want 1", n)
				}
				h = HeatmapOf(Filter{})
				if n != 2 {
					t.Fatalf("the heatmap paired %d times after the page of All, want 1: its recent days", n-1)
				}
			}
			calls := 0
			for _, d := range h.Days {
				calls += d.Calls
			}
			if calls != 400 {
				t.Fatalf("the heatmap counted %d calls, want 400", calls)
			}
		})
	}
}
