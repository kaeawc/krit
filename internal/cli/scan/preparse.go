package scan

import (
	"context"
	"os"

	"github.com/kaeawc/krit/internal/pipeline"
)

// startPreparse begins parsing the sources while the oracle JVM runs, when a
// cold scan is certain to parse every file: no analysis cache can serve any of
// them. The parse phase takes the result only if its own input matches.
func (r *runner) startPreparse(ctx context.Context) {
	if !r.preparseEligible() {
		return
	}
	r.setupParseCachesOnce()
	in := r.projectInput()
	r.preparse = pipeline.StartPreparse(ctx, in.Args, in.Host)
}

// preparseEligible reports whether every file will be parsed after the oracle.
func (r *runner) preparseEligible() bool {
	if len(r.files) == 0 || r.depthPreset == DepthThorough || len(parseCustomRuleJars(*r.f.CustomRuleJars)) > 0 {
		return false
	}
	if *r.f.NoCache {
		return true
	}
	if r.cacheFilePath == "" {
		return false
	}
	_, err := os.Stat(r.cacheFilePath)
	return os.IsNotExist(err)
}
