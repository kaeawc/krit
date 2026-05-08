package lsp

import (
	"fmt"
	"net/url"
	"strings"
)

// JARScheme is the URI scheme for synthetic JAR-source documents that the
// LSP server can produce for goto-def / find-references results pointing
// into compiled dependencies.
//
// The shape is:
//
//	krit-jar:///{artifact}/{version}/{path-with-slashes}.kt
//
// The artifact and version are recovered from the JAR filename when
// possible (e.g. kotlinx-coroutines-core-1.7.3.jar →
// artifact=kotlinx-coroutines-core, version=1.7.3) and the path-with-slashes
// is the FQN of the requested symbol. The original on-disk JAR path and
// FQN are encoded as query parameters so the server can locate them again
// without re-deriving anything from the URI hierarchy.
const JARScheme = "krit-jar"

// JARRef identifies a single symbol inside a JAR for the purposes of
// decompilation. Both fields are required.
type JARRef struct {
	JARPath string // absolute filesystem path to the .jar
	FQN     string // fully qualified name of the requested declaration
}

// BuildJARURI returns a synthetic LSP URI for a JAR-resolved declaration.
// The visible path embeds the artifact, version, and FQN-as-path so editors
// render a recognisable label, while the JAR path and FQN ride along as
// query parameters for round-trip parsing.
func BuildJARURI(ref JARRef) string {
	artifact, version := splitJARFilename(ref.JARPath)
	pathPart := strings.ReplaceAll(ref.FQN, ".", "/") + ".kt"

	q := url.Values{}
	q.Set("jar", ref.JARPath)
	q.Set("fqn", ref.FQN)

	u := url.URL{
		Scheme:   JARScheme,
		Path:     "/" + artifact + "/" + version + "/" + pathPart,
		RawQuery: q.Encode(),
	}
	return u.String()
}

// ParseJARURI is the inverse of BuildJARURI. It returns an error for any
// URI that does not use the krit-jar scheme or that is missing the
// required jar/fqn query parameters.
func ParseJARURI(uri string) (JARRef, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return JARRef{}, fmt.Errorf("parse uri: %w", err)
	}
	if u.Scheme != JARScheme {
		return JARRef{}, fmt.Errorf("not a %s URI: %q", JARScheme, uri)
	}
	q := u.Query()
	jar := q.Get("jar")
	fqn := q.Get("fqn")
	if jar == "" || fqn == "" {
		return JARRef{}, fmt.Errorf("missing jar or fqn in %q", uri)
	}
	return JARRef{JARPath: jar, FQN: fqn}, nil
}

// IsJARURI reports whether uri uses the krit-jar scheme. It is cheaper than
// ParseJARURI and tolerates malformed query strings.
func IsJARURI(uri string) bool {
	return strings.HasPrefix(uri, JARScheme+":")
}

// splitJARFilename extracts {artifact, version} from a JAR filename of the
// form "<artifact>-<version>.jar". When no version-looking suffix is
// present it returns the bare basename as the artifact and "unknown" as
// the version. Inputs without ".jar" suffix are passed through verbatim
// for the artifact slot.
func splitJARFilename(jarPath string) (artifact, version string) {
	base := jarPath
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".jar")
	if base == "" {
		return "unknown", "unknown"
	}
	// Find a "-N..." suffix where N is a digit — the conventional Maven
	// version separator. Falls back to the whole basename when absent.
	for i := 0; i < len(base); i++ {
		if base[i] != '-' {
			continue
		}
		if i+1 < len(base) && base[i+1] >= '0' && base[i+1] <= '9' {
			return base[:i], base[i+1:]
		}
	}
	return base, "unknown"
}
