package api

import "regexp"

// CategoryPrecompile is the api.Rule.Category for compiler-class
// diagnostics. See docs/precompile/taxonomy.md for the full contract;
// precompile_conventions_test.go enforces it.
const CategoryPrecompile = "precompile"

// PrecompileIDPattern matches every valid precompile rule ID.
var PrecompileIDPattern = regexp.MustCompile(`^K\d{4}-[A-Z][A-Za-z0-9]+$`)

// PrecompileMetaIDPattern matches the K9000-K9999 meta range, which is
// exempt from the SeverityError floor (meta diagnostics signal
// infrastructure conditions, not user defects).
var PrecompileMetaIDPattern = regexp.MustCompile(`^K9\d{3}-[A-Z][A-Za-z0-9]+$`)

func IsPrecompileID(id string) bool {
	return PrecompileIDPattern.MatchString(id)
}

func IsPrecompileMetaID(id string) bool {
	return PrecompileMetaIDPattern.MatchString(id)
}
