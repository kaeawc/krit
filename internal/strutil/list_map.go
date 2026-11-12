package strutil

import "strings"

// DecodeListMap parses whitespace-separated "key=a,b,c" entries into a map of ordered lists. A key with no values
// maps to an empty, non-nil list so presence checks keep working. It keeps large static tables out of compiled
// init code.
func DecodeListMap(spec string) map[string][]string {
	fields := strings.Fields(spec)
	out := make(map[string][]string, len(fields))
	for _, f := range fields {
		key, vals, _ := strings.Cut(f, "=")
		if vals == "" {
			out[key] = []string{}
			continue
		}
		out[key] = strings.Split(vals, ",")
	}
	return out
}
