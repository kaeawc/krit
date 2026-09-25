package serve

import (
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
)

// A delegated `krit --fir` scan gets the FIR pass installed as the
// pipeline's findings post pass; without --fir the daemon path is unchanged.
func TestBuildProjectInput_InstallsFirPostPassOnlyForFir(t *testing.T) {
	root := t.TempDir()
	state := newDaemonState(root)
	cfg := config.NewConfig()
	state.cachedConfig = cfg

	off, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{Paths: []string{root}, NoCache: true}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if off.Host.FindingsPostPass != nil {
		t.Fatal("no --fir: the daemon must not install a findings post pass")
	}

	on, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{Paths: []string{root}, NoCache: true, Fir: true}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if on.Host.FindingsPostPass == nil {
		t.Fatal("--fir: the daemon must install the FIR pass as the findings post pass")
	}
}
