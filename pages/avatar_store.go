package pages

import (
	"sync"
	"time"

	"go-media-storage/avatar_generator"
)

var (
	avatarCacheOnce sync.Once
	avatarCache     *avatar_generator.Cache
)

// avatarStore returns the process-wide avatar cache. One instance is shared by
// every route so that a warm avatar is a warm avatar everywhere.
func avatarStore() *avatar_generator.Cache {
	avatarCacheOnce.Do(func() {
		avatarCache = avatar_generator.NewCache(avatar_generator.DefaultCacheConfig())
	})
	return avatarCache
}

// heavySize is the edge length above which a render allocates enough memory to
// be worth throttling. At 4096 the RGBA buffer alone is 64 MiB.
const heavySize = 1024

// heavyRenderSlots bounds how many oversized avatars are drawn at once, and
// heavyWait bounds how long a request waits for a slot before giving up.
const (
	heavyRenderSlots = 2
	heavyWait        = 5 * time.Second
)

var heavySlots = make(chan struct{}, heavyRenderSlots)

// acquireHeavySlot blocks until a slot is free or the wait expires.
func acquireHeavySlot() bool {
	timer := time.NewTimer(heavyWait)
	defer timer.Stop()

	select {
	case heavySlots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	}
}

func releaseHeavySlot() {
	<-heavySlots
}
