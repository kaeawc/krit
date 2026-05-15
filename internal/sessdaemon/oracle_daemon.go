package sessdaemon

import (
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/oracle"
)

// oracleStarter abstracts JVM-subprocess construction so tests can
// substitute a fake without spinning up a real krit-types daemon. A
// nil daemon with nil error means "not configured in this environment"
// (e.g. krit-types.jar missing); the caller leaves oracle disabled
// without latching for-the-process so a deferred install can recover.
type oracleStarter interface {
	Start(scanPaths []string) (*oracle.Daemon, error)
}

type defaultOracleStarter struct{}

func (defaultOracleStarter) Start(scanPaths []string) (*oracle.Daemon, error) {
	if oracle.FindJar(scanPaths) == "" {
		return nil, nil
	}
	return oracle.InvokeDaemon(scanPaths, false)
}

// oracleDaemonState tracks the resident *oracle.Daemon's recovery
// semantics. The Daemon handle itself lives on session.OracleDaemon
// (issue #207) so Session.Close stops the JVM during daemon shutdown.
type oracleDaemonState struct {
	disabled bool
	starter  oracleStarter
}

// ensureOracle returns the resident *oracle.Daemon, lazy-starting it
// on first request and recovering after a mid-life crash. Returns nil
// when the JVM is not configured in this environment, when a previous
// failure has latched disabled, or when both attempts of the
// retry-once budget fail.
//
// Called from handleAnalyze, which holds s.mu for the duration of the
// request — no additional synchronisation here.
func (s *Server) ensureOracle(scanPaths []string) *oracle.Daemon {
	if s.oracle.disabled {
		return nil
	}

	if d := s.session.OracleDaemon; d != nil {
		if err := d.Ping(); err == nil {
			return d
		}
		_ = d.Close()
		s.session.OracleDaemon = nil
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		d, err := s.oracle.starter.Start(scanPaths)
		if err == nil {
			if d == nil {
				// Starter reported "not configured" — no retry, no latch.
				return nil
			}
			s.session.OracleDaemon = d
			return d
		}
		// Cold-start failures often clear on a second attempt (JDK
		// file lock, transient port conflict on the krit-types socket).
		lastErr = err
	}
	s.oracle.disabled = true
	fmt.Fprintf(os.Stderr, "krit-daemon: oracle daemon: start failed twice (%v); disabling for this daemon lifetime\n", lastErr)
	return nil
}
