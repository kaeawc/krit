package firchecks

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// FirInvocationFingerprint combines the classpath with the checker binary
// identity and the enabled rule set. Stat avoids hashing the large fat jar.
func FirInvocationFingerprint(classpath []string, jarPath string, rules []string) string {
	jarIdentity := jarPath + ":missing"
	if info, err := os.Stat(jarPath); err == nil {
		jarIdentity = fmt.Sprintf("%s:%d:%d", jarPath, info.Size(), info.ModTime().UnixNano())
	}
	ids := slices.Clone(rules)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	return hashutil.HashHex([]byte(ClasspathFingerprint(classpath) + "\x00" + jarIdentity + "\x00" + strings.Join(ids, "\x00")))
}
