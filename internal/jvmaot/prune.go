package jvmaot

import (
	"regexp"
	"time"

	"github.com/kaeawc/krit/internal/cacheprune"
)

const (
	// keepCacheGroups is how many recently modified per-jar cache groups always survive.
	keepCacheGroups = 6
	// minCacheGroupAge keeps groups touched within a day, which may be in use.
	minCacheGroupAge = 24 * time.Hour
	// pruneInterval limits the directory scan to once a day.
	pruneInterval = 24 * time.Hour
)

var cacheGroupRE = regexp.MustCompile(`^krit-types-([0-9a-f]{12})(?:[-.]|$)`)

// PruneStaleCaches removes the AppCDS and AOT files of old jars from cacheDir.
// Each jar's files form one group keyed by its content hash. The newest groups,
// groups under a day old, and the group for protectHash always survive, so only
// caches of superseded jars are removed. A removed cache is rebuilt on next use.
func PruneStaleCaches(cacheDir, protectHash string) {
	cacheprune.Prune(cacheDir, cacheprune.Options{
		Keep:     keepCacheGroups,
		MinAge:   minCacheGroupAge,
		Interval: pruneInterval,
		Protect:  []string{protectHash},
		GroupOf: func(name string) (string, bool) {
			m := cacheGroupRE.FindStringSubmatch(name)
			if m == nil {
				return "", false
			}
			return m[1], true
		},
	})
}
