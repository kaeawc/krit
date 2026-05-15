package scan

import (
	"context"
	"errors"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/scanner"
)

// Flags is the bundle of CLI options NewSession may consult. Alias so
// the same *scanFlags can flow from the one-shot Run path and from the
// daemon's startup wiring without an additional adapter type.
type Flags = *scanFlags

// Session owns the long-lived state the daemon constructs once at
// startup and reuses across every scan request. One-shot CLI builds a
// fresh Session per request and Close drains it on exit. Runner phases
// write back into Session so daemon callers skip the rebuild on warm
// invocations.
type Session struct {
	Workspace             *pipeline.WorkspaceState
	AnalysisCache         *cache.Cache
	AnalysisCacheFilePath string
	ParseCache            *scanner.ParseCache
	XMLParseCache         *android.XMLParseCache
	ResourceCache         *android.ResourceIndexCache
	AndroidProject        *android.Project
	LibraryFacts          *librarymodel.Facts
	OracleDaemon          *oracle.Daemon
	repoDir               string
	closed                bool
}

// NewSession returns a Session rooted at repoDir. Caches and project
// model fields beyond Workspace/LibraryFacts are populated by the scan
// runner; daemon callers may pre-populate them to skip rebuild work.
//
// ctx and flags are accepted for the daemon scaffolding that builds on
// this refactor; the one-shot CLI path does not consult them yet.
func NewSession(ctx context.Context, repoDir string, flags Flags) (*Session, error) {
	_ = ctx
	_ = flags
	return &Session{
		Workspace:    pipeline.NewWorkspaceState(repoDir),
		LibraryFacts: librarymodel.DefaultFacts(),
		repoDir:      repoDir,
	}, nil
}

// RepoDir reports the repository root the session was constructed against.
func (s *Session) RepoDir() string {
	if s == nil {
		return ""
	}
	return s.repoDir
}

// Close is safe on a nil receiver and idempotent so callers can defer
// it alongside the runner's mid-scan flushCaches without double-close
// hazards.
func (s *Session) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	var errs []error
	if s.ParseCache != nil {
		if err := s.ParseCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.XMLParseCache != nil {
		if err := s.XMLParseCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.ResourceCache != nil {
		if err := s.ResourceCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.OracleDaemon != nil {
		s.OracleDaemon.Close()
	}
	return errors.Join(errs...)
}
