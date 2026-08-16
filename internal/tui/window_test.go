package tui

import "testing"

// Every combination, because the arithmetic is easy to get subtly wrong and
// the symptom is an empty menu. Whatever the heights, offset, cursor and budget, the window has
// to contain the cursor and must not overflow the budget.
func TestWindowAlwaysHoldsTheCursor(t *testing.T) {
	heights := [][]int{
		{1, 1, 1, 1, 1, 1, 1, 1},
		{2, 1, 1, 2, 1, 1, 2, 1},
		{1, 2, 3, 1, 1, 1, 2, 1},
		{3, 3, 3, 3},
		{1},
	}

	bad := 0
	for _, tall := range heights {
		for budget := 1; budget <= 12; budget++ {
			for offset := 0; offset < len(tall); offset++ {
				for cursor := 0; cursor < len(tall); cursor++ {
					start, end := window(tall, offset, cursor, budget)

					if cursor < start || cursor >= end {
						if bad++; bad < 8 {
							t.Errorf("cursor %d outside [%d,%d) budget=%d tall=%v offset=%d",
								cursor, start, end, budget, tall, offset)
						}
						continue
					}

					used := 0
					if start > 0 {
						used++
					}
					if end < len(tall) {
						used++
					}
					for i := start; i < end; i++ {
						used += tall[i]
					}
					if used > budget && end-start > 1 {
						if bad++; bad < 8 {
							t.Errorf("window [%d,%d) uses %d of budget %d tall=%v",
								start, end, used, budget, tall)
						}
					}
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("total failures: %d", bad)
	}
}
