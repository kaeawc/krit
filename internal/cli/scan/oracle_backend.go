package scan

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
)

// resolveOracleBackend implements the priority chain runner_state.go
// uses when picking the JVM daemon: the `--oracle-backend` flag wins
// over the `oracle.backend` value in krit.yml, which in turn wins over
// [oracle.DefaultBackend]. Returning a typed error keeps malformed
// values from silently falling back to the default — see
// [oracle.ParseBackend].
//
// Kept on its own so the resolution rule has one tested home; both
// the CLI plumbing and oracle-backend-resolution unit tests call this
// directly.
func resolveOracleBackend(flagValue string, cfg *config.Config) (oracle.Backend, error) {
	raw := flagValue
	if raw == "" && cfg != nil {
		raw = cfg.Oracle().Backend
	}
	return oracle.ParseBackend(raw)
}
