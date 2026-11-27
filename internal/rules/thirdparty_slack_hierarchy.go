package rules

// Class-hierarchy helpers shared by the slack-lints ports. Lint resolves supertypes against the classpath; krit
// walks the source resolver's class index, resolving each supertype name through the declaring file's imports.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// slackSupertypeFQN resolves a supertype name as the resolver records it for class info (the text written in
// source, without type arguments) to an FQN: through info's file's imports (so `View` there is the View it imports,
// and `RecyclerView.ItemDecoration` the imported RecyclerView's nested class), as a class of info's package, or as a
// source class of that name. A name it cannot resolve is returned as written.
func slackSupertypeFQN(resolver typeinfer.TypeResolver, info *typeinfer.ClassInfo, super string) string {
	if i := strings.IndexByte(super, '<'); i >= 0 {
		super = super[:i]
	}
	super = strings.TrimSpace(super)
	head, rest, nested := strings.Cut(super, ".")
	// ResolveImport reads only the path of the file it is given. The resolver keeps no import table for Java
	// files, so a Java class's imports are read from its source.
	declaring := &scanner.File{Path: info.File, Language: scanner.LangKotlin}
	fqn := resolver.ResolveImport(head, declaring)
	if !strings.Contains(fqn, ".") && strings.HasSuffix(info.File, ".java") {
		fqn = slackJavaFileImports(info.File)[head]
	}
	if strings.Contains(fqn, ".") {
		if nested {
			return fqn + "." + rest
		}
		return fqn
	}
	if nested && head != "" && head[0] >= 'a' && head[0] <= 'z' {
		return super
	}
	if pkg := slackClassPackage(info); pkg != "" {
		if same := resolver.ClassHierarchy(pkg + "." + super); same != nil {
			return same.FQN
		}
	}
	if source := resolver.ClassHierarchy(super); source != nil && source.FQN != "" {
		return source.FQN
	}
	return super
}

// slackClassPackage returns the package of a source class, or "".
func slackClassPackage(info *typeinfer.ClassInfo) string {
	if i := strings.LastIndex(info.FQN, "."+info.Name); i >= 0 {
		pkg := info.FQN[:i]
		// A nested class's prefix is its outer class; packages are lower case.
		for strings.Contains(pkg, ".") {
			last := pkg[strings.LastIndexByte(pkg, '.')+1:]
			if last == "" || last[0] < 'A' || last[0] > 'Z' {
				break
			}
			pkg = pkg[:strings.LastIndexByte(pkg, '.')]
		}
		return pkg
	}
	return ""
}

// slackClassExtends reports whether the class fqn is target or, through source classes the resolver indexes, a
// subtype of it. A library class other than target ends the walk, so only source subtypes of target are found.
func slackClassExtends(resolver typeinfer.TypeResolver, fqn, target string) bool {
	return slackClassExtendsSeen(resolver, fqn, target, map[string]bool{})
}

func slackClassExtendsSeen(resolver typeinfer.TypeResolver, fqn, target string, seen map[string]bool) bool {
	if fqn == target || (target == "android.view.View" && slackIsFrameworkView(fqn)) {
		return true
	}
	if resolver == nil || fqn == "" || seen[fqn] {
		return false
	}
	seen[fqn] = true
	info := resolver.ClassHierarchy(fqn)
	if info == nil || info.FQN != fqn {
		return false
	}
	for _, super := range slackRecordedSupertypes(info) {
		if slackClassExtendsSeen(resolver, slackSupertypeFQN(resolver, info, super), target, seen) {
			return true
		}
	}
	return false
}

// slackRecordedSupertypes returns a class's direct supertypes as written in source. The resolver's Supertypes
// qualifies a Kotlin simple name with any same-named source class, imported or not, so DirectSupertypes, which keeps
// the written names, is preferred when the resolver records it.
func slackRecordedSupertypes(info *typeinfer.ClassInfo) []string {
	if len(info.DirectSupertypes) > 0 {
		return info.DirectSupertypes
	}
	return info.Supertypes
}

// slackIsFrameworkView reports whether a library class is one of the Android framework or AndroidX View classes krit
// knows by name (TextView, LinearLayout, RecyclerView, ...), so a project class extending it is a View.
func slackIsFrameworkView(fqn string) bool {
	if !strings.HasPrefix(fqn, "android.") && !strings.HasPrefix(fqn, "androidx.") && !strings.HasPrefix(fqn, "com.google.android.material.") {
		return false
	}
	_, ok := androidKnownViewSimpleNames[fqn[strings.LastIndexByte(fqn, '.')+1:]]
	return ok
}

var (
	slackJavaImportRe    = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_][\w.]*)\s*;`)
	slackJavaImportCache sync.Map // path -> map[string]string
)

// slackJavaFileImports returns a Java source file's single-type imports by simple name. The result is memoized by
// path, size and modification time, so a long-lived daemon or LSP session rereads an edited file.
func slackJavaFileImports(path string) map[string]string {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", path, st.Size(), st.ModTime().UnixNano())
	if v, ok := slackJavaImportCache.Load(key); ok {
		return v.(map[string]string)
	}
	out := map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		for _, m := range slackJavaImportRe.FindAllStringSubmatch(string(data), -1) {
			out[m[1][strings.LastIndexByte(m[1], '.')+1:]] = m[1]
		}
	}
	slackJavaImportCache.Store(key, out)
	return out
}
