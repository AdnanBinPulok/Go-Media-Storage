package avatar_generator

import (
	"runtime"
	"sync"
)

// Row-band parallelism for the drawing pass.
//
// Every draw call works on its own pixels: shape rows never overlap, and the
// body, face and eyes are painted in sequence. Splitting rows across goroutines
// therefore writes disjoint memory with no synchronisation beyond the wait, and
// the result is bit-identical to the sequential loop.
const (
	// maxDrawWorkers caps fans-out so a big server does not spawn dozens of
	// goroutines for a 256px avatar.
	maxDrawWorkers = 8

	// minBandRows is the smallest useful band. Below this the goroutine
	// handoff costs more than the rows themselves.
	minBandRows = 24
)

// parallelBands runs fn(lo, hi) over contiguous slices of [y0, y1).
//
// Small ranges run inline, so tiny avatars (where the whole body is a handful
// of rows) never pay for goroutines.
func parallelBands(y0, y1 int, fn func(lo, hi int)) {
	rows := y1 - y0
	if rows <= 0 {
		return
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > maxDrawWorkers {
		workers = maxDrawWorkers
	}
	// Never create a band thinner than minBandRows.
	if byRows := rows / minBandRows; byRows < workers {
		workers = byRows
	}
	if workers < 2 {
		fn(y0, y1)
		return
	}

	band := (rows + workers - 1) / workers

	var wg sync.WaitGroup
	for lo := y0; lo < y1; lo += band {
		hi := min(lo+band, y1)
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			fn(lo, hi)
		}(lo, hi)
	}
	wg.Wait()
}
