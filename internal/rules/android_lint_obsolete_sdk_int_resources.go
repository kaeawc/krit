package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ObsoleteSdkIntResourceRule mirrors the resource half of Lint's ObsoleteSdkInt: a `-vNN` resource folder whose
// version the minSdk already meets, and a `tools:targetApi` that does the same.
type ObsoleteSdkIntResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the module's minSdk comes from its build file or manifest.
func (r *ObsoleteSdkIntResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// ObsoleteSdkIntManifestRule mirrors the manifest half of Lint's ObsoleteSdkInt: a `tools:targetApi` the minSdk
// already meets.
type ObsoleteSdkIntManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the module's minSdk comes from its build file or manifest.
func (r *ObsoleteSdkIntManifestRule) Confidence() float64 { return api.ConfidenceMedium }

// sdkFolderTypes are the resource folder types Lint checks for an unnecessary version. raw folders are left out.
var sdkFolderTypes = map[string]bool{
	"anim": true, "animator": true, "color": true, "drawable": true, "font": true, "interpolator": true, "layout": true,
	"menu": true, "mipmap": true, "navigation": true, "transition": true, "values": true, "xml": true,
}

// Folder qualifiers in the order Android requires, each as the category number it holds.
const (
	sdkQualLocale  = 2
	sdkQualVersion = 22
)

var (
	sdkQualifierPatterns = []struct {
		category int
		re       *regexp.Regexp
	}{
		{0, regexp.MustCompile(`^mcc\d{3}$`)},
		{1, regexp.MustCompile(`^mnc\d{1,3}$`)},
		{3, regexp.MustCompile(`^(ldrtl|ldltr)$`)},
		{4, regexp.MustCompile(`^sw\d+dp$`)},
		{5, regexp.MustCompile(`^w\d+dp$`)},
		{6, regexp.MustCompile(`^h\d+dp$`)},
		{7, regexp.MustCompile(`^(small|normal|large|xlarge)$`)},
		{8, regexp.MustCompile(`^(long|notlong)$`)},
		{9, regexp.MustCompile(`^(round|notround)$`)},
		{10, regexp.MustCompile(`^(widecg|nowidecg)$`)},
		{11, regexp.MustCompile(`^(highdr|lowdr)$`)},
		{12, regexp.MustCompile(`^(port|land)$`)},
		{13, regexp.MustCompile(`^(car|desk|television|appliance|watch|vrheadset)$`)},
		{14, regexp.MustCompile(`^(night|notnight)$`)},
		{15, regexp.MustCompile(`^(ldpi|mdpi|hdpi|xhdpi|xxhdpi|xxxhdpi|nodpi|anydpi|tvdpi|\d+dpi)$`)},
		{16, regexp.MustCompile(`^(notouch|finger)$`)},
		{17, regexp.MustCompile(`^(keysexposed|keyshidden|keyssoft)$`)},
		{18, regexp.MustCompile(`^(nokeys|qwerty|12key)$`)},
		{19, regexp.MustCompile(`^(navexposed|navhidden)$`)},
		{20, regexp.MustCompile(`^(nonav|dpad|trackball|wheel)$`)},
		{21, regexp.MustCompile(`^\d+x\d+$`)},
		{sdkQualVersion, regexp.MustCompile(`^v\d+$`)},
	}
	sdkLanguageRe = regexp.MustCompile(`^[a-z]{2,3}$`)
	sdkRegionRe   = regexp.MustCompile(`^r[A-Z]{2}$`)
	sdkBcp47Re    = regexp.MustCompile(`^b\+[A-Za-z0-9+]+$`)
)

// sdkQualifierCategory returns the category of a folder qualifier, and whether it is a region that must follow a language.
func sdkQualifierCategory(q string) (category int, region bool) {
	for _, p := range sdkQualifierPatterns {
		if p.re.MatchString(q) {
			return p.category, false
		}
	}
	switch {
	case sdkRegionRe.MatchString(q):
		return sdkQualLocale, true
	case sdkLanguageRe.MatchString(q), sdkBcp47Re.MatchString(q):
		return sdkQualLocale, false
	}
	return -1, false
}

// sdkFolderVersion splits a resource folder name such as `layout-land-v19` into the folder without its version
// qualifier and the version. It reports false for a folder with no version or with qualifiers out of order.
func sdkFolderVersion(name string) (base string, version int, ok bool) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 || !sdkFolderTypes[parts[0]] {
		return "", 0, false
	}
	last, prevLanguage := -1, false
	for _, q := range parts[1:] {
		category, region := sdkQualifierCategory(q)
		switch {
		case category < 0:
			return "", 0, false
		case region && !prevLanguage:
			return "", 0, false
		case !region && category <= last:
			return "", 0, false
		}
		prevLanguage = category == sdkQualLocale && !region && !strings.HasPrefix(q, "b+")
		last = category
	}
	if last != sdkQualVersion {
		return "", 0, false
	}
	v, err := strconv.Atoi(parts[len(parts)-1][1:])
	if err != nil {
		return "", 0, false
	}
	return strings.Join(parts[:len(parts)-1], "-"), v, true
}

// sdkCodeNames maps the lowercase Build.VERSION_CODES names, and L for Lollipop, to their levels.
var sdkCodeNames = func() map[string]int {
	out := map[string]int{"l": 21}
	for fq, v := range sdkVersionCodes {
		out[strings.ToLower(fq[strings.LastIndex(fq, ".")+1:])] = int(v)
	}
	return out
}()

// sdkTargetAPILevel reads a tools:targetApi value: a level, or a Build.VERSION_CODES name in any case.
func sdkTargetAPILevel(value string) (int, bool) {
	if v, ok := sdkCodeNames[strings.ToLower(value)]; ok {
		return v, true
	}
	if value == "" || value[0] == '0' || strings.TrimLeft(value, "0123456789") != "" {
		return 0, false
	}
	v, err := strconv.Atoi(value)
	return v, err == nil && v >= 1
}

// obsoleteSdkMinSdkAt returns the minSdk of the module that owns the path, or 0 when none is declared.
func obsoleteSdkMinSdkAt(ctx *api.Context, path string) int {
	if levels := android.ResolveSDKLevels(path); levels.MinSdk > 0 {
		return levels.MinSdk
	}
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil {
		return facts.Profile.MinSdkVersion
	}
	return 0
}

func sdkTargetAPIMessage(level int) string {
	return "Unnecessary; `SDK_INT` is always >= " + strconv.Itoa(level)
}

func (r *ObsoleteSdkIntResourceRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	dirs := append([]string(nil), ctx.ResourceIndex.ResDirs...)
	sort.Strings(dirs)
	for _, dir := range dirs {
		minSdk := obsoleteSdkMinSdkAt(ctx, filepath.Join(dir, "x"))
		if minSdk < 1 {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			folder := filepath.Join(dir, e.Name())
			r.checkFolder(ctx, folder, e.Name(), minSdk)
			r.checkTargetAPI(ctx, folder, e.Name(), minSdk)
		}
	}
}

func (r *ObsoleteSdkIntResourceRule) checkFolder(ctx *api.Context, path, name string, minSdk int) {
	base, version, ok := sdkFolderVersion(name)
	if !ok || version <= 1 || version > minSdk {
		return
	}
	ctx.Emit(scanner.Finding{File: path, Line: 1, Col: 1, Message: fmt.Sprintf(
		"This folder configuration (`v%d`) is unnecessary; `minSdkVersion` is %d. Merge all the resources in this folder into `%s`.",
		version, minSdk, base)})
}

// checkTargetAPI reports tools:targetApi attributes in the XML files of one resource folder.
func (r *ObsoleteSdkIntResourceRule) checkTargetAPI(ctx *api.Context, folder, name string, minSdk int) {
	if base, _, _ := strings.Cut(name, "-"); base == "raw" {
		return
	}
	files, err := os.ReadDir(folder)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".xml") {
			continue
		}
		path := filepath.Join(folder, f.Name())
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "targetApi") {
			continue
		}
		root, err := android.ParseXMLAST(context.Background(), data)
		if err != nil || root == nil {
			continue
		}
		prefixes := sdkToolsPrefixes(root)
		walkXMLToolsIgnore(root, func(n *android.XMLNode, tokens toolsIgnoreSet) {
			if tokens.silences("ObsoleteSdkInt") {
				return
			}
			for _, a := range n.Attrs {
				if level, ok := sdkTargetAPILevel(a.Value); ok && level <= minSdk && sdkIsToolsTargetAPI(a.Name, prefixes) {
					ctx.Emit(scanner.Finding{File: path, Line: a.Line, Col: a.Col, Message: sdkTargetAPIMessage(level)})
				}
			}
		})
	}
}

// sdkToolsPrefixes returns the prefixes the root element binds to the tools namespace.
func sdkToolsPrefixes(root *android.XMLNode) map[string]bool {
	out := map[string]bool{}
	for _, a := range root.Attrs {
		if prefix, ok := strings.CutPrefix(a.Name, "xmlns:"); ok && a.Value == lintToolsURI {
			out[prefix] = true
		}
	}
	return out
}

func sdkIsToolsTargetAPI(name string, prefixes map[string]bool) bool {
	prefix, local, ok := strings.Cut(name, ":")
	return ok && local == "targetApi" && prefixes[prefix]
}

func (r *ObsoleteSdkIntManifestRule) check(ctx *api.Context) {
	if ctx.Manifest == nil {
		return
	}
	minSdk := ctx.Manifest.MinSDK
	if minSdk < 1 {
		minSdk = obsoleteSdkMinSdkAt(ctx, ctx.Manifest.Path)
	}
	if minSdk < 1 {
		return
	}
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		for _, a := range n.Attrs {
			if local, ok := strings.CutPrefix(a.Name, "tools:"); ok && local == "targetApi" {
				if level, ok := sdkTargetAPILevel(a.Value); ok && level <= minSdk {
					emitManifestAtAttr(ctx, a, sdkTargetAPIMessage(level))
				}
			}
		}
	})
}
