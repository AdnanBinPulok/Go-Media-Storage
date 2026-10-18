package pages

import (
	"sync"
	"time"

	"go-media-storage/avatar_generator"
	"go-media-storage/settings"
)

var (
	avatarCacheOnce sync.Once
	avatarCache     *avatar_generator.Cache
)

// avatarStore returns the process-wide avatar cache. One instance is shared by
// every route so that a warm avatar is a warm avatar everywhere.
func avatarStore() *avatar_generator.Cache {
	avatarCacheOnce.Do(func() {
		cfg := avatar_generator.DefaultCacheConfig()
		if ttl := settings.CachingConfigInstance.AvatarMemoryTTL; ttl > 0 {
			cfg.TTL = time.Duration(ttl) * time.Second
		}
		avatarCache = avatar_generator.NewCache(cfg)
	})
	return avatarCache
}

// heavySize is the edge length above which a render allocates enough memory to
// be worth throttling. At 4096 the RGBA buffer alone is 64 MiB.
const heavySize = 1024

// AvatarDefaultSize resolves the configured default against the generator's
// accepted range. A zero or out-of-range env value falls back safely instead of
// producing a broken render.
func AvatarDefaultSize() int {
	size := settings.AvatarConfigInstance.DefaultSize
	if size == 0 {
		size = avatar_generator.DefaultSize
	}
	return ClampAvatarSize(size)
}

// ClampAvatarSize pins size to the generator's [MinSize, MaxSize] range.
func ClampAvatarSize(size int) int {
	switch {
	case size < avatar_generator.MinSize:
		return avatar_generator.MinSize
	case size > avatar_generator.MaxSize:
		return avatar_generator.MaxSize
	default:
		return size
	}
}

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
