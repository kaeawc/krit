package rules

import "github.com/kaeawc/krit/internal/strutil"

// decodeListMap is strutil.DecodeListMap under the name the generated locale tables call.
func decodeListMap(spec string) map[string][]string { return strutil.DecodeListMap(spec) }
