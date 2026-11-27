// Package fixture is input for TestAnalyzeBuildInfoReach: resource, resource-source and icon rules that reach the
// build-file helpers (ruleslinter.BuildInfoHelpers) in the ways the gate follows and the ways it does not.
package fixture

import (
	"os"
	"path/filepath"
)

type ResourceBuildInfoReads struct{ Lookups, Files []string }

type ResourceBuildInfoReader interface {
	ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads
}

type Context struct{ ResDir string }

// The build-file helpers, by the names the gate lists.

func lookupManifestGradleInfo(path string) int { return readManifestGradleInfo(filepath.Dir(path)) }

func readManifestGradleInfo(dir string) int {
	_, _ = os.ReadFile(filepath.Join(dir, "build.gradle.kts"))
	return 0
}

func catalogPluginIDs(string) []string                 { return nil }
func versionCatalogsFor(string) []string               { return nil }
func catalogDeclaresDependency(string) bool            { return false }
func resourceModuleMinSdk(resFile string) int          { return lookupManifestGradleInfo(resFile) }
func readModuleIdentity(string) string                 { return "" }
func readModuleVectorSettings(string) bool             { return false }
func gradleModulePath(string) string                   { return "" }
func gradleSettingsCandidates(string) []string         { return nil }
func manifestRoot(string) any                          { return nil }
func translucentManifestPath(string) string            { return "" }
func unusedResourceRootFiles(string) []string          { return nil }
func walkFiles(dir string, visit func(string))         { visit(dir) }
func plainResourceCheck(ctx *Context) bool             { return ctx.ResDir != "" }
func emit(*Context, string)                            {}
func minSdkFromValue(f func(string) int, p string) int { return f(p) }

// DirectRule calls resourceModuleMinSdk without implementing ResourceBuildInfoReader.
type DirectRule struct{}

func (r *DirectRule) check(ctx *Context) {
	if resourceModuleMinSdk(ctx.ResDir) < 21 {
		emit(ctx, "low")
	}
}

// DeclaredRule calls resourceModuleMinSdk and implements ResourceBuildInfoReader.
type DeclaredRule struct{}

func (r *DeclaredRule) check(ctx *Context) { _ = resourceModuleMinSdk(ctx.ResDir) }

func (r *DeclaredRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	return ResourceBuildInfoReads{Lookups: []string{resDir}}
}

// TransitiveRule reaches resourceModuleMinSdk through a local helper.
type TransitiveRule struct{}

func (r *TransitiveRule) check(ctx *Context) {
	if transitiveMinSdk(ctx) > 0 {
		emit(ctx, "x")
	}
}

func transitiveMinSdk(ctx *Context) int { return resourceModuleMinSdk(ctx.ResDir) }

// OtherTypeRule reaches resourceModuleMinSdk through a method on another concrete type.
type OtherTypeRule struct{ facts moduleFacts }

type moduleFacts struct{}

func (moduleFacts) minSdk(resDir string) int { return resourceModuleMinSdk(resDir) }

func (r *OtherTypeRule) check(ctx *Context) { _ = r.facts.minSdk(ctx.ResDir) }

// ClosureRule reaches resourceModuleMinSdk inside a callback it passes to a walker.
type ClosureRule struct{}

func (r *ClosureRule) check(ctx *Context) {
	walkFiles(ctx.ResDir, func(path string) { _ = resourceModuleMinSdk(path) })
}

// FuncValueRule passes resourceModuleMinSdk as a value: naming the function in reachable code is followed.
type FuncValueRule struct{}

func (r *FuncValueRule) check(ctx *Context) { _ = minSdkFromValue(resourceModuleMinSdk, ctx.ResDir) }

// FieldValueRule calls a function stored in a field elsewhere: the gate does not follow it (a known limit).
type FieldValueRule struct{ minSdk func(string) int }

func (r *FieldValueRule) check(ctx *Context) { _ = r.minSdk(ctx.ResDir) }

// NewFieldValueRule is not reachable from FieldValueRule.check.
func NewFieldValueRule() *FieldValueRule { return &FieldValueRule{minSdk: resourceModuleMinSdk} }

// InterfaceRule calls through an interface that moduleFacts implements: class hierarchy analysis follows it.
type InterfaceRule struct{ src minSdkSource }

type minSdkSource interface{ minSdk(resDir string) int }

// StringerRule calls an interface of another package that no package type implementing it reads build files through.
type StringerRule struct{ s interface{ String() string } }

func (r *StringerRule) check(ctx *Context) { emit(ctx, r.s.String()) }

type label struct{}

func (label) String() string { return "" }

func (r *InterfaceRule) check(ctx *Context) { _ = r.src.minSdk(ctx.ResDir) }

// UnusedReaderRule implements ResourceBuildInfoReader but reaches no helper.
type UnusedReaderRule struct{}

func (r *UnusedReaderRule) check(ctx *Context) { _ = plainResourceCheck(ctx) }

func (r *UnusedReaderRule) ResourceBuildInfoReads(string) ResourceBuildInfoReads {
	return ResourceBuildInfoReads{}
}

// CoveredRule reaches manifestRoot only through wearIsWearResDir, whose reads the res-dir fingerprint covers
// (ruleslinter.BuildInfoCovered).
type CoveredRule struct{}

func (r *CoveredRule) check(ctx *Context) { _ = wearIsWearResDir(ctx.ResDir) }

func wearIsWearResDir(resDir string) bool { return manifestRoot(resDir) != nil }

// PlainRule reads no build file.
type PlainRule struct{}

func (r *PlainRule) check(ctx *Context) { _ = plainResourceCheck(ctx) }

// IconRule reaches manifestRoot in a scope whose keys take no build info, whatever it implements.
type IconRule struct{}

func (r *IconRule) check(ctx *Context) { _ = manifestRoot(ctx.ResDir) }

func (r *IconRule) ResourceBuildInfoReads(string) ResourceBuildInfoReads {
	return ResourceBuildInfoReads{}
}

// checkPackageFunc is a Check given as a package function.
func checkPackageFunc(ctx *Context) { _ = readModuleIdentity(ctx.ResDir) }

// checkFuncVar is a Check given as a package variable.
var checkFuncVar = func(ctx *Context) { _ = gradleModulePath(ctx.ResDir) }

// register mimics api.Register with a Check closure.
func register(check func(*Context)) { _ = check }

func init() {
	register(func(ctx *Context) { _ = readModuleVectorSettings(ctx.ResDir) }) // closureCheckLine
	register(checkFuncVar)
}
