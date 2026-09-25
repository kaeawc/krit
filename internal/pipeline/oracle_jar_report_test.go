package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/oracle"
)

// A failed jar download must surface as a warning — before, a missing
// krit-fir jar silently disabled the default oracle. A download that was
// never attempted stays verbose-only.
func TestReportMissingOracleJar(t *testing.T) {
	skipped := fmt.Errorf("krit-fir.jar not found (%w: dev build)", oracle.ErrJarDownloadSkipped)
	failed := errors.New("download krit-fir.jar: http 404")
	for _, tc := range []struct {
		name        string
		err         error
		verbose     bool
		wantWarning string
		wantVerbose string
	}{
		{"failed download warns", failed, false, "warning: type oracle disabled: download krit-fir.jar", ""},
		{"skipped is silent", skipped, false, "", ""},
		{"skipped is verbose", skipped, true, "", "verbose: type oracle disabled: krit-fir.jar not found"},
		{"nil is silent", nil, true, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var warn, verbose bytes.Buffer
			in := IndexInput{Verbose: tc.verbose, Reporter: &diag.Reporter{Warning: &warn, Verbose: &verbose}}
			in.reportMissingOracleJar(tc.err)
			if !strings.Contains(warn.String(), tc.wantWarning) || (tc.wantWarning == "" && warn.Len() > 0) {
				t.Errorf("warning output = %q, want %q", warn.String(), tc.wantWarning)
			}
			if !strings.Contains(verbose.String(), tc.wantVerbose) || (tc.wantVerbose == "" && verbose.Len() > 0) {
				t.Errorf("verbose output = %q, want %q", verbose.String(), tc.wantVerbose)
			}
		})
	}
}
