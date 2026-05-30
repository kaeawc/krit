package pipeline

import (
	"context"

	"github.com/kaeawc/krit/internal/scanner"
)

// parseFilesByPath parses exactly the supplied Kotlin and Java source paths
// into *scanner.File, reusing the daemon's resident-file cache and disk parse
// cache (and the normal generated-file filtering + suppression-index install)
// via ParsePhase.Run.
//
// It exists for the affected-set replay path: a warm run parses only the dirty
// (cache-miss) files, so an affected reverse-dependency file that was not
// edited has no parsed *scanner.File available. This materializes those files
// on demand so they can be re-dispatched, instead of falling back to a full
// project dispatch.
//
// Both path lists reach ParseInput as non-nil slices. That is load-bearing:
// ParsePhase.Run treats a nil KotlinPaths/JavaPaths as "collect the whole
// project", whereas a non-nil empty slice parses nothing for that language.
// (Java is additionally gated on the active rules driving Java collection, the
// same as every other ParsePhase.Run caller.)
func parseFilesByPath(ctx context.Context, args ProjectArgs, host ProjectHostState, kotlinPaths, javaPaths []string) ([]*scanner.File, []*scanner.File, error) {
	if len(kotlinPaths) == 0 && len(javaPaths) == 0 {
		return nil, nil, nil
	}
	res, err := ParsePhase{Workers: args.Workers}.Run(ctx, ParseInput{
		Config:           args.Config,
		Paths:            args.Paths,
		ActiveRules:      args.ActiveRules,
		IncludeGenerated: args.IncludeGenerated,
		KotlinPaths:      append([]string{}, kotlinPaths...),
		JavaPaths:        append([]string{}, javaPaths...),
		Workers:          args.Workers,
		Reporter:         host.Reporter,
		Tracker:          host.Tracker,
		ParseCache:       host.ParseCache,
		ResidentFiles:    host.ResidentFiles,
	})
	if err != nil {
		return nil, nil, err
	}
	return res.KotlinFiles, res.JavaFiles, nil
}
