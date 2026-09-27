package oracle

// UnionSourceDirs keeps convention-derived roots first, followed by model
// roots, without compiling the same directory twice.
func UnionSourceDirs(convention, model []string) []string {
	seen := make(map[string]bool, len(convention)+len(model))
	var out []string
	for _, dir := range append(append([]string(nil), convention...), model...) {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}
