package rules

import (
	"bytes"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TrulyRandomRule ports Android Lint's TrulyRandom (SecureRandomGeneratorDetector).
// Android 4.3 and older seeded the platform PRNG poorly, so key generation,
// signing, encryption and SecureRandom use is flagged when the project supports
// API 18 or lower. Like Lint, it reports once per project, at the first
// matching call, and stays quiet when the project applies the NativeCrypto
// workaround (Class.forName of the NativeCrypto provider class).
//
// Matched calls follow the detector: getInstance and constructors of
// KeyGenerator, KeyPairGenerator, KeyAgreement and SecureRandom; wrap and
// unwrap on SSLEngine; Cipher.init with ENCRYPT_MODE or WRAP_MODE. The
// detector also lists Signature.initSign, but under the owner
// javax/crypto/Signature, which does not exist (the class is
// java.security.Signature), so it never fires and is not ported.
//
// The krit-only hardcoded-seed check that used this name is HardcodedRandomSeedRule.
type TrulyRandomRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receiver types are
// inferred from imports, not resolved.
func (r *TrulyRandomRule) Confidence() float64 { return api.ConfidenceMedium }

const (
	trulyRandomMessage = "Potentially insecure random numbers on Android 4.3 and older. " +
		"Read https://android-developers.blogspot.com/2013/08/some-securerandom-thoughts.html for more info."
	// trulyRandomMaxMinSdk is the highest minSdk that still gets the warning.
	trulyRandomMaxMinSdk  = 18
	trulyRandomWorkaround = "org.apache.harmony.xnet.provider.jsse.NativeCrypto"
)

// trulyRandomGetInstanceOwners are the classes whose getInstance and constructors Lint flags.
var trulyRandomGetInstanceOwners = map[string]string{
	"KeyGenerator":     "javax.crypto",
	"KeyPairGenerator": "java.security",
	"KeyAgreement":     "javax.crypto",
	"SecureRandom":     "java.security",
}

type trulyRandomHit struct {
	file      *scanner.File
	line, col int
}

func (r *TrulyRandomRule) check(ctx *api.Context) {
	if trulyRandomProjectMinSdk(ctx) > trulyRandomMaxMinSdk {
		return
	}
	files := append([]*scanner.File(nil), ctx.ParsedFiles...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	var first *trulyRandomHit
	for _, file := range files {
		if file == nil || file.FlatTree == nil || scanner.IsTestFile(file.Path) {
			continue
		}
		if bytes.Contains(file.Content, []byte(trulyRandomWorkaround)) && bytes.Contains(file.Content, []byte("forName")) {
			return
		}
		if first == nil {
			first = trulyRandomFirstHit(file)
		}
	}
	if first != nil {
		ctx.Emit(r.Finding(first.file, first.line, first.col, trulyRandomMessage))
	}
}

// trulyRandomProjectMinSdk returns the Gradle minSdk, or 1 (Lint's default) when unknown.
func trulyRandomProjectMinSdk(ctx *api.Context) int {
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil && facts.Profile.MinSdkVersion > 0 {
		return facts.Profile.MinSdkVersion
	}
	return 1
}

// trulyRandomFirstHit returns the first matching call of file in source order.
func trulyRandomFirstHit(file *scanner.File) *trulyRandomHit {
	imports := trulyRandomImports(file)
	var best *trulyRandomHit
	bestByte := uint32(0)
	consider := func(at uint32) {
		if best != nil && file.FlatStartByte(at) >= bestByte {
			return
		}
		best = &trulyRandomHit{file: file, line: file.FlatRow(at) + 1, col: file.FlatCol(at) + 1}
		bestByte = file.FlatStartByte(at)
	}
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if at := trulyRandomKotlinCall(file, call, imports); at != 0 {
			consider(at)
		}
	})
	file.FlatWalkNodes(0, "method_invocation", func(call uint32) {
		if at := trulyRandomJavaCall(file, call, imports); at != 0 {
			consider(at)
		}
	})
	file.FlatWalkNodes(0, "object_creation_expression", func(node uint32) {
		if trulyRandomJavaConstructs(file, node, imports) {
			consider(node)
		}
	})
	return best
}

// trulyRandomImports holds a file's import paths, with wildcard imports kept as "pkg.*".
type trulyRandomImportSet map[string]bool

func trulyRandomImports(file *scanner.File) trulyRandomImportSet {
	out := trulyRandomImportSet{}
	add := func(node uint32) {
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimSuffix(strings.TrimPrefix(text, "import"), ";")
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "static "))
		if i := strings.Index(text, " as "); i >= 0 {
			text = text[:i]
		}
		out[strings.TrimSpace(text)] = true
	}
	file.FlatWalkNodes(0, "import_header", add)
	file.FlatWalkNodes(0, "import_declaration", add)
	return out
}

// has reports whether the simple class name resolves to pkg.name through an import or wildcard.
func (i trulyRandomImportSet) has(pkg, name string) bool {
	return i[pkg+"."+name] || i[pkg+".*"]
}

// ownerOf resolves a receiver expression to the simple name of a known owner class, or "".
func (i trulyRandomImportSet) ownerOf(receiver string) string {
	receiver = strings.Join(strings.Fields(receiver), "")
	for name, pkg := range trulyRandomGetInstanceOwners {
		if receiver == pkg+"."+name || receiver == name && i.has(pkg, name) {
			return name
		}
	}
	return ""
}

func (i trulyRandomImportSet) isCipher(receiver string) bool {
	return receiver == "javax.crypto.Cipher" || receiver == "Cipher" && i.has("javax.crypto", "Cipher")
}

// trulyRandomCipherEncryptArg reports whether the first Cipher.init argument is ENCRYPT_MODE or WRAP_MODE.
func trulyRandomCipherEncryptArg(arg string, imports trulyRandomImportSet) bool {
	arg = strings.Join(strings.Fields(arg), "")
	if i := strings.LastIndex(arg, "."); i >= 0 {
		return (arg[i+1:] == "ENCRYPT_MODE" || arg[i+1:] == "WRAP_MODE") && imports.isCipher(arg[:i])
	}
	return (arg == "1" || arg == "3") && imports.has("javax.crypto", "Cipher")
}

func (i trulyRandomImportSet) hasSSLEngine() bool { return i.has("javax.net.ssl", "SSLEngine") }

// trulyRandomKotlinCall returns the node to report when a Kotlin call matches, else 0.
func trulyRandomKotlinCall(file *scanner.File, call uint32, imports trulyRandomImportSet) uint32 {
	name := flatCallExpressionName(file, call)
	navExpr, args := flatCallExpressionParts(file, call)
	receiver := ""
	if navExpr != 0 {
		text := file.FlatNodeText(navExpr)
		if dot := strings.LastIndex(text, "."); dot >= 0 {
			receiver = text[:dot]
		}
	}
	switch {
	case navExpr == 0 && trulyRandomGetInstanceOwners[name] != "":
		if imports.has(trulyRandomGetInstanceOwners[name], name) {
			return call
		}
	case name == "getInstance" && imports.ownerOf(receiver) != "":
		return call
	case name == "init" && receiver != "" && args != 0:
		if arg := flatPositionalValueArgument(file, args, 0); arg != 0 {
			if expr := flatValueArgumentExpression(file, arg); expr != 0 && trulyRandomCipherEncryptArg(file.FlatNodeText(expr), imports) {
				return call
			}
		}
	case (name == "wrap" || name == "unwrap") && receiver != "" && imports.hasSSLEngine():
		return call
	}
	return 0
}

// trulyRandomJavaCall returns the node to report when a Java method call matches, else 0.
func trulyRandomJavaCall(file *scanner.File, call uint32, imports trulyRandomImportSet) uint32 {
	name := secureRandomJavaMethodName(file, call)
	recvNode := secureRandomJavaMethodReceiver(file, call)
	receiver := ""
	if recvNode != 0 {
		receiver = file.FlatNodeText(recvNode)
	}
	switch {
	case name == "getInstance" && imports.ownerOf(receiver) != "":
		return call
	case name == "init" && receiver != "":
		if args, ok := file.FlatFindChild(call, "argument_list"); ok {
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatIsNamed(arg) {
					if trulyRandomCipherEncryptArg(file.FlatNodeText(arg), imports) {
						return call
					}
					break
				}
			}
		}
	case (name == "wrap" || name == "unwrap") && receiver != "" && imports.hasSSLEngine():
		return call
	}
	return 0
}

// trulyRandomJavaConstructs reports whether node is `new <known owner>(...)`.
func trulyRandomJavaConstructs(file *scanner.File, node uint32, imports trulyRandomImportSet) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			return imports.ownerOf(file.FlatNodeText(child)) != ""
		}
	}
	return false
}
