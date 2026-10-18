// Package avatar_generator draws deterministic PNG avatars from any string id.
//
// The same (id, size, padding) always produces byte-identical output: there is
// no clock, no global RNG, and no shared mutable state. That property is what
// makes the result safe to cache and safe to serve with a long max-age.
//
// Basic use:
//
//	png, err := avatar_generator.GeneratePNG("550e8400-e29b-41d4-a716-446655440000", 256, avatar_generator.DefaultPaddingPct)
//
// With caching:
//
//	cache := avatar_generator.NewCache(avatar_generator.DefaultCacheConfig())
//	png, hit, err := cache.PNG(id, 256, avatar_generator.DefaultPaddingPct)
//
// The package is safe for concurrent use.
package avatar_generator
