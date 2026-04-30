package deadcode

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// ProjectFinding describes a declaration unreachable from project roots.
type ProjectFinding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	FQN        string `json:"fqn"`
	Visibility string `json:"visibility"`
	Reason     string `json:"reason"`
	Module     string `json:"module,omitempty"`
}

// ProjectOptions configures project-level dead-code analysis.
type ProjectOptions struct {
	// Roots are user-supplied additional reachability roots, identified by
	// FQN or simple name.
	Roots []string
	// Workers controls parser parallelism. Defaults to runtime.NumCPU().
	Workers int
	// Paths restricts scanning to these directories. Defaults to ScanRoot.
	Paths []string
}

// Annotations that mark a declaration as a reachability root.
var rootAnnotations = []string{
	"@HiltAndroidApp",
	"@AndroidEntryPoint",
	"@HiltViewModel",
	"@HiltWorker",
	"@Serializable",
	"@Provides",
	"@Inject",
	"@Singleton",
	"@Module",
	"@Test",
}

var servicesEntryRe = regexp.MustCompile(`(?m)^([\w.$]+)`)

// AnalyzeProject performs project-level reachability dead-code analysis.
// It discovers Gradle modules at scanRoot (best effort), parses Kotlin and Java
// files under opts.Paths, identifies reachability roots, and reports any
// non-private declarations not reachable from those roots.
func AnalyzeProject(scanRoot string, opts ProjectOptions) ([]ProjectFinding, error) {
	if opts.Workers < 1 {
		opts.Workers = runtime.NumCPU()
	}
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{scanRoot}
	}

	graph, _ := module.DiscoverModules(scanRoot)
	if graph != nil {
		_ = module.ParseAllDependencies(graph)
	}

	ktPaths, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		return nil, err
	}
	files, _ := scanner.ScanFiles(ktPaths, opts.Workers)
	javaPaths, err := scanner.CollectJavaFiles(paths, nil)
	if err != nil {
		return nil, err
	}
	javaFiles, _ := scanner.ScanJavaFiles(javaPaths, opts.Workers)
	idx := scanner.BuildIndex(files, opts.Workers, javaFiles...)

	allFiles := make([]*scanner.File, 0, len(files)+len(javaFiles))
	allFiles = append(allFiles, files...)
	allFiles = append(allFiles, javaFiles...)
	fileByPath := make(map[string]*scanner.File, len(allFiles))
	for _, f := range allFiles {
		fileByPath[f.Path] = f
	}

	rootNames := buildSimpleNameSet(opts.Roots)
	if graph != nil {
		for _, m := range graph.Modules {
			collectManifestRoots(m.Dir, rootNames)
			collectServiceLoaderRoots(m.Dir, rootNames)
		}
	}
	collectManifestRoots(scanRoot, rootNames)
	collectServiceLoaderRoots(scanRoot, rootNames)

	nameToSyms := make(map[string][]int, len(idx.Symbols))
	declSites := make(map[string]map[string]map[int]bool)
	for i, s := range idx.Symbols {
		nameToSyms[s.Name] = append(nameToSyms[s.Name], i)
		if s.FQN != "" && s.FQN != s.Name {
			nameToSyms[s.FQN] = append(nameToSyms[s.FQN], i)
		}
		byName := declSites[s.File]
		if byName == nil {
			byName = make(map[string]map[int]bool)
			declSites[s.File] = byName
		}
		lines := byName[s.Name]
		if lines == nil {
			lines = make(map[int]bool)
			byName[s.Name] = lines
		}
		lines[s.Line] = true
	}

	fileRefs := make(map[string]map[string]bool)
	for _, r := range idx.References {
		if r.InComment {
			continue
		}
		// Skip refs that coincide with a declaration site — the
		// tree-sitter walk emits the declared name as both a symbol
		// and an identifier reference at the same line.
		if byName, ok := declSites[r.File]; ok {
			if lines, ok := byName[r.Name]; ok && lines[r.Line] {
				continue
			}
		}
		set := fileRefs[r.File]
		if set == nil {
			set = make(map[string]bool)
			fileRefs[r.File] = set
		}
		set[r.Name] = true
	}

	reachable := make([]bool, len(idx.Symbols))
	queue := make([]int, 0, 64)
	for i, s := range idx.Symbols {
		if isRootSymbol(s, fileByPath[s.File], rootNames, graph) {
			reachable[i] = true
			queue = append(queue, i)
		}
	}

	visitedFiles := make(map[string]bool)
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		s := idx.Symbols[i]
		if visitedFiles[s.File] {
			continue
		}
		visitedFiles[s.File] = true
		for name := range fileRefs[s.File] {
			for _, j := range nameToSyms[name] {
				if !reachable[j] {
					reachable[j] = true
					queue = append(queue, j)
				}
			}
		}
	}

	var findings []ProjectFinding
	seen := make(map[string]bool)
	for i, s := range idx.Symbols {
		if reachable[i] {
			continue
		}
		if !isReportableSymbol(s) {
			continue
		}
		modPath := ""
		if graph != nil {
			modPath = graph.FileToModule(s.File)
		}
		key := s.File + ":" + strconv.Itoa(s.Line) + ":" + s.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		findings = append(findings, ProjectFinding{
			File:       s.File,
			Line:       s.Line,
			Kind:       s.Kind,
			Name:       s.Name,
			FQN:        projectSymbolFQN(modPath, s),
			Visibility: s.Visibility,
			Reason:     "no-callers",
			Module:     modPath,
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Name < findings[j].Name
	})
	return findings, nil
}

func isReportableSymbol(s scanner.Symbol) bool {
	if s.Name == "" {
		return false
	}
	if s.Visibility == "private" {
		return false
	}
	if s.IsOverride {
		return false
	}
	if s.IsTest || s.IsMain {
		return false
	}
	if s.Language == scanner.LangJava {
		return isReportableJavaProjectSymbol(s)
	}
	return true
}

func isReportableJavaProjectSymbol(s scanner.Symbol) bool {
	if s.Visibility != "public" {
		return false
	}
	switch s.Kind {
	case "class", "interface", "enum", "record", "annotation":
		return s.FQN != ""
	case "method":
		return s.Owner != "" && s.FQN != "" && s.Signature != ""
	default:
		return false
	}
}

func isRootSymbol(s scanner.Symbol, f *scanner.File, rootNames map[string]bool, graph *module.ModuleGraph) bool {
	if s.IsMain || s.IsTest {
		return true
	}
	if rootNames[s.Name] {
		return true
	}
	if s.FQN != "" && rootNames[s.FQN] {
		return true
	}
	if graph != nil && s.Visibility != "private" {
		modPath := graph.FileToModule(s.File)
		if mod := graph.Modules[modPath]; mod != nil && mod.IsPublished {
			return true
		}
	}
	if f != nil && len(f.Content) > 0 && s.StartByte >= 0 && s.EndByte > s.StartByte && s.EndByte <= len(f.Content) {
		body := f.Content[s.StartByte:s.EndByte]
		for _, ann := range rootAnnotations {
			if containsAnnotation(body, ann) {
				return true
			}
		}
	}
	return false
}

// containsAnnotation looks for ann at a token boundary so that
// "@Serializable" doesn't also match "@SerializableContract".
func containsAnnotation(body []byte, ann string) bool {
	s := string(body)
	for {
		i := strings.Index(s, ann)
		if i < 0 {
			return false
		}
		end := i + len(ann)
		if end == len(s) || !isIdentRune(s[end]) {
			return true
		}
		s = s[end:]
	}
}

func isIdentRune(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func collectManifestRoots(dir string, names map[string]bool) {
	if dir == "" {
		return
	}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) != "AndroidManifest.xml" {
			return nil
		}
		manifest, parseErr := android.ParseManifest(path)
		if parseErr != nil || manifest == nil {
			return nil
		}
		for _, c := range manifest.AllComponents() {
			addSimpleName(names, c.Component.Name)
		}
		return nil
	})
}

func collectServiceLoaderRoots(dir string, names map[string]bool) {
	if dir == "" {
		return
	}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// META-INF/services/<service-name>: each non-comment line is an FQN.
		if !strings.Contains(filepath.ToSlash(path), "/META-INF/services/") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, m := range servicesEntryRe.FindAllSubmatch(data, -1) {
			line := strings.TrimSpace(string(m[1]))
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			addSimpleName(names, line)
		}
		return nil
	})
}

func buildSimpleNameSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		addSimpleName(out, v)
	}
	return out
}

func addSimpleName(out map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	out[value] = true
	if dot := strings.LastIndex(value, "."); dot >= 0 && dot+1 < len(value) {
		out[value[dot+1:]] = true
	}
}

func buildFQN(modPath, name string) string {
	if modPath == "" || modPath == "root" {
		return name
	}
	return modPath + "." + name
}

func projectSymbolFQN(modPath string, s scanner.Symbol) string {
	if s.FQN != "" {
		return s.FQN
	}
	return buildFQN(modPath, s.Name)
}
