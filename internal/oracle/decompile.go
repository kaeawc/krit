package oracle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Decompiler turns a {jar, fqn} pair into Kotlin-shaped source text. The
// production implementation will RPC into krit-types and use the Kotlin
// Analysis API's KotlinDecompiledLightClassSupport. Until that lands the
// LSP path uses a signature-stub renderer driven entirely from data the
// oracle already collects in Data.Dependencies — enough for editors
// to render a recognisable virtual document.
type Decompiler interface {
	Decompile(jarPath, fqn string) (string, error)
}

type DaemonDecompiler struct {
	Daemon *Daemon
}

func (d DaemonDecompiler) Decompile(jarPath, fqn string) (string, error) {
	if d.Daemon == nil {
		return "", fmt.Errorf("decompile: daemon unavailable")
	}
	return d.Daemon.DecompileJar(jarPath, fqn)
}

type FallbackDecompiler struct {
	Primary  Decompiler
	Fallback Decompiler
}

func (d FallbackDecompiler) Decompile(jarPath, fqn string) (string, error) {
	if d.Primary != nil {
		if src, err := d.Primary.Decompile(jarPath, fqn); err == nil && src != "" {
			return src, nil
		}
	}
	if d.Fallback == nil {
		return "", fmt.Errorf("decompile: no fallback decompiler")
	}
	return d.Fallback.Decompile(jarPath, fqn)
}

// DecompileCache wraps a Decompiler with a content-addressed disk cache.
// JAR contents are immutable per content hash, so cached decompiles never
// need invalidating: a cache entry under {jar-sha}/{fqn}.kt is correct
// forever for that JAR's bytes.
type DecompileCache struct {
	root       string
	source     Decompiler
	mu         sync.Mutex
	jarHashes  map[string]string // jarPath → sha
	missingJAR map[string]bool   // jarPath → true once we've logged that the JAR is missing
}

// NewDecompileCache wires a cache rooted at root. root is created lazily on
// first write; callers typically point this at ".krit/jar-decompile" inside
// the project.
func NewDecompileCache(root string, source Decompiler) *DecompileCache {
	return &DecompileCache{
		root:       root,
		source:     source,
		jarHashes:  make(map[string]string),
		missingJAR: make(map[string]bool),
	}
}

// Get returns the decompiled source for {jarPath, fqn}, reading from disk
// when available and otherwise calling the underlying Decompiler and
// persisting the result. When the JAR file itself is missing, Get falls
// back to the supplied source — this is the "signature-stub" path the
// proposal describes for unresolved classpath entries.
func (c *DecompileCache) Get(jarPath, fqn string) (string, error) {
	if fqn == "" {
		return "", fmt.Errorf("decompile: empty fqn")
	}
	jarHash, jarOK, err := c.hashJAR(jarPath)
	if err != nil {
		return "", err
	}
	if !jarOK {
		// JAR isn't on disk; render directly without caching since we
		// can't key the entry by content hash.
		return c.source.Decompile(jarPath, fqn)
	}

	cachePath := filepath.Join(c.root, jarHash, fqnToFilename(fqn))
	if data, err := os.ReadFile(cachePath); err == nil {
		return string(data), nil
	}

	src, err := c.source.Decompile(jarPath, fqn)
	if err != nil {
		return "", err
	}
	if err := writeDecompiledSourceAtomic(cachePath, []byte(src)); err != nil {
		// Cache failures are non-fatal — the caller still gets a result.
		return src, nil //nolint:nilerr // see comment: cache write failure surfaces nothing to caller, source is already decompiled
	}
	return src, nil
}

// hashJAR returns the sha256 of the JAR's content (as a lowercase hex
// string) plus a boolean indicating whether the JAR was readable. Hashes
// are memoised: repeated lookups for the same path are O(1) after the
// first, which matters because LSP traffic touches stdlib JARs frequently.
func (c *DecompileCache) hashJAR(jarPath string) (string, bool, error) {
	c.mu.Lock()
	if h, ok := c.jarHashes[jarPath]; ok {
		c.mu.Unlock()
		return h, true, nil
	}
	c.mu.Unlock()

	f, err := os.Open(jarPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.mu.Lock()
			c.missingJAR[jarPath] = true
			c.mu.Unlock()
			return "", false, nil
		}
		return "", false, fmt.Errorf("open jar: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false, fmt.Errorf("hash jar: %w", err)
	}
	hex := hex.EncodeToString(h.Sum(nil))

	c.mu.Lock()
	c.jarHashes[jarPath] = hex
	c.mu.Unlock()
	return hex, true, nil
}

// JARMissing reports whether Get has previously observed jarPath to be
// unreadable. The LSP server uses this to log unresolved classpath entries
// without spamming the log on every request.
func (c *DecompileCache) JARMissing(jarPath string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.missingJAR[jarPath]
}

func fqnToFilename(fqn string) string {
	return strings.ReplaceAll(fqn, ".", "/") + ".kt"
}

// writeDecompiledSourceAtomic creates any missing parent directories
// and then delegates to fsutil.WriteFileAtomic so the cached
// decompiled source survives a hard crash. The previous local helper
// renamed without fsync of the tempfile or the parent directory,
// which on power loss could leave the cache showing a clean write
// while the file vanished or reverted on remount.
func writeDecompiledSourceAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// SignatureStubDecompiler renders Kotlin-shaped source from the oracle's
// own Dependencies map. It is deliberately schematic: it shows the kind,
// modifiers, supertypes, and members so a navigation result is recognisable
// in an editor, but it does not reconstruct method bodies. When the real
// KAA-backed decompiler is wired through krit-types this implementation
// remains useful as a fallback for entries the daemon can't resolve.
type SignatureStubDecompiler struct {
	// Lookup returns the Class for an FQN, or nil if unknown.
	Lookup func(fqn string) *Class
}

// Decompile satisfies Decompiler.
func (d *SignatureStubDecompiler) Decompile(jarPath, fqn string) (string, error) {
	cls := (*Class)(nil)
	if d.Lookup != nil {
		cls = d.Lookup(fqn)
	}
	return RenderSignatureStub(jarPath, fqn, cls), nil
}

// RenderSignatureStub produces the placeholder Kotlin source for one FQN.
// Exposed for tests and for callers that want to render without going
// through the cache.
func RenderSignatureStub(jarPath, fqn string, cls *Class) string {
	pkg, simple := splitFQN(fqn)
	var b strings.Builder

	b.WriteString("// Decompiled stub generated by krit.\n")
	b.WriteString("// Source: ")
	if jarPath == "" {
		b.WriteString("(unresolved classpath entry)\n")
	} else {
		b.WriteString(jarPath)
		b.WriteByte('\n')
	}
	b.WriteString("// Bodies are omitted; signatures only.\n\n")

	if pkg != "" {
		fmt.Fprintf(&b, "package %s\n\n", pkg)
	}

	if cls == nil {
		fmt.Fprintf(&b, "// %s — declaration not present in oracle index.\n", fqn)
		fmt.Fprintf(&b, "class %s\n", simple)
		return b.String()
	}

	if len(cls.Annotations) > 0 {
		for _, a := range cls.Annotations {
			fmt.Fprintf(&b, "@%s\n", a)
		}
	}
	writeClassHeader(&b, cls, simple)

	members := append([]*Member(nil), cls.Members...)
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Kind != members[j].Kind {
			// properties first, then functions, for stable readability.
			return members[i].Kind < members[j].Kind
		}
		return members[i].Name < members[j].Name
	})
	if len(members) == 0 {
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(" {\n")
	for _, m := range members {
		writeMember(&b, m)
	}
	b.WriteString("}\n")
	return b.String()
}

func writeClassHeader(b *strings.Builder, cls *Class, simple string) {
	if cls.Visibility != "" && cls.Visibility != "public" {
		fmt.Fprintf(b, "%s ", cls.Visibility)
	}
	if cls.IsSealed {
		b.WriteString("sealed ")
	} else if cls.IsAbstract {
		b.WriteString("abstract ")
	} else if cls.IsOpen {
		b.WriteString("open ")
	}
	if cls.IsData {
		b.WriteString("data ")
	}
	kind := cls.Kind
	if kind == "" {
		kind = "class"
	}
	fmt.Fprintf(b, "%s %s", kind, simple)
	if len(cls.TypeParameters) > 0 {
		fmt.Fprintf(b, "<%s>", strings.Join(cls.TypeParameters, ", "))
	}
	if len(cls.Supertypes) > 0 {
		fmt.Fprintf(b, " : %s", strings.Join(cls.Supertypes, ", "))
	}
}

func writeMember(b *strings.Builder, m *Member) {
	b.WriteString("    ")
	if m.Visibility != "" && m.Visibility != "public" {
		fmt.Fprintf(b, "%s ", m.Visibility)
	}
	if m.IsAbstract {
		b.WriteString("abstract ")
	}
	if m.IsOverride {
		b.WriteString("override ")
	}
	switch m.Kind {
	case "property":
		fmt.Fprintf(b, "val %s: %s", m.Name, formatType(m.ReturnType, m.Nullable))
	default:
		fmt.Fprintf(b, "fun %s(", m.Name)
		for i, p := range m.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(b, "%s: %s", p.Name, formatType(p.Type, p.Nullable))
		}
		b.WriteString(")")
		if m.ReturnType != "" && m.ReturnType != "kotlin.Unit" {
			fmt.Fprintf(b, ": %s", formatType(m.ReturnType, m.Nullable))
		}
	}
	b.WriteByte('\n')
}

func formatType(t string, nullable bool) string {
	if t == "" {
		t = "Any"
	}
	if nullable {
		return t + "?"
	}
	return t
}

func splitFQN(fqn string) (pkg, simple string) {
	i := strings.LastIndex(fqn, ".")
	if i < 0 {
		return "", fqn
	}
	return fqn[:i], fqn[i+1:]
}
