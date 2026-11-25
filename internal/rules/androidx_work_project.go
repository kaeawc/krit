package rules

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of the project-wide checks bundled in androidx.work:work-runtime 2.12.0. Lint collects facts in visitClass
// and visitMethodCall and reports in afterCheckRootProject; these rules do the same over every parsed Kotlin and
// Java file of the project (Lint skips test sources, and so do they).

const (
	workApplicationClass   = "android.app.Application"
	workConfigProvider     = "androidx.work.Configuration.Provider"
	workJobServiceClass    = "android.app.job.JobService"
	workSystemJobService   = "androidx.work.impl.background.systemjob.SystemJobService"
	workListenableWorker   = "androidx.work.ListenableWorker"
	workBadConfigMessage   = "Expected Application subtype to implement Configuration.Provider"
	workJobIDRangeMessage  = "Specify a valid range of job id's for `WorkManager` to use."
	workPublicWorkerSuffix = " needs to be public"
)

// workLibrarySupertypes maps library classes that project code commonly extends to their supertypes, so a project
// class's hierarchy reaches the framework types the detectors look for.
var workLibrarySupertypes = map[string][]string{
	"androidx.work.Worker":                                     {workListenableWorker},
	"androidx.work.CoroutineWorker":                            {workListenableWorker},
	"androidx.work.RxWorker":                                   {workListenableWorker},
	"androidx.work.rxjava3.RxWorker":                           {workListenableWorker},
	"androidx.work.multiprocess.RemoteListenableWorker":        {workListenableWorker},
	"androidx.work.multiprocess.RemoteCoroutineWorker":         {"androidx.work.multiprocess.RemoteListenableWorker"},
	"androidx.multidex.MultiDexApplication":                    {workApplicationClass},
	"androidx.work.impl.background.systemjob.SystemJobService": {workJobServiceClass},
}

// workClass is one class-like declaration of the project.
type workClass struct {
	file     *scanner.File
	idx      uint32
	fqn      string
	supers   [][]string // per written supertype, the FQNs it may name
	abstract bool
	iface    bool
	public   bool
}

// workProject indexes the project's classes by FQN.
type workProject struct {
	classes []*workClass
	byFQN   map[string]*workClass
}

// workProjectFiles returns the parsed main-source files sorted by path, the order Lint visits them in.
func workProjectFiles(ctx *api.Context) []*scanner.File {
	files := make([]*scanner.File, 0, len(ctx.ParsedFiles))
	for _, f := range ctx.ParsedFiles {
		if f != nil && f.FlatTree != nil && !scanner.IsTestFile(f.Path) &&
			(f.Language == scanner.LangKotlin || f.Language == scanner.LangJava) {
			files = append(files, f)
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func buildWorkProject(files []*scanner.File) *workProject {
	p := &workProject{byFQN: map[string]*workClass{}}
	for _, file := range files {
		pkg := filePackageName(file)
		for _, kind := range []string{"class_declaration", "object_declaration", "interface_declaration"} {
			file.FlatWalkNodes(0, kind, func(idx uint32) {
				c := workClassOf(file, pkg, idx)
				if c != nil {
					p.classes = append(p.classes, c)
					p.byFQN[c.fqn] = c
				}
			})
		}
	}
	// Report order is document order within each file, files in path order.
	sort.SliceStable(p.classes, func(i, j int) bool {
		if p.classes[i].file != p.classes[j].file {
			return p.classes[i].file.Path < p.classes[j].file.Path
		}
		return p.classes[i].file.FlatStartByte(p.classes[i].idx) < p.classes[j].file.FlatStartByte(p.classes[j].idx)
	})
	return p
}

func workClassOf(file *scanner.File, pkg string, idx uint32) *workClass {
	name := lintClassNameNode(file, idx)
	if name == 0 && file.FlatType(idx) == "object_declaration" {
		if n, ok := file.FlatFindChild(idx, "type_identifier"); ok {
			name = n
		}
	}
	if name == 0 {
		return nil
	}
	parts := []string{file.FlatNodeText(name)}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "interface_declaration":
			if n := lintClassNameNode(file, cur); n != 0 {
				parts = append([]string{file.FlatNodeText(n)}, parts...)
			} else if n, ok := file.FlatFindChild(cur, "type_identifier"); ok {
				parts = append([]string{file.FlatNodeText(n)}, parts...)
			}
		case "function_declaration", "method_declaration", "lambda_literal", "object_literal":
			// A local class has no stable qualified name; Lint does not see it as a project type either.
			return nil
		}
	}
	fqn := strings.Join(parts, ".")
	if pkg != "" {
		fqn = pkg + "." + fqn
	}
	c := &workClass{file: file, idx: idx, fqn: fqn}
	c.iface = file.FlatType(idx) == "interface_declaration" || (file.FlatType(idx) == "class_declaration" && lintDeclaresInterface(file, idx))
	c.abstract = c.iface || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "sealed")
	c.public = workIsJVMPublic(file, idx)
	for _, super := range lintSupertypeNames(file, idx) {
		c.supers = append(c.supers, workResolveTypeRef(file, pkg, super))
	}
	return c
}

// workIsJVMPublic reports whether the class compiles to a public JVM class. Kotlin classes are public unless private
// or protected (internal is public in bytecode); Java classes need the public modifier, except members of an
// interface, which are implicitly public.
func workIsJVMPublic(file *scanner.File, idx uint32) bool {
	if file.Language == scanner.LangKotlin {
		return !file.FlatHasModifier(idx, "private") && !file.FlatHasModifier(idx, "protected")
	}
	if file.FlatHasModifier(idx, "public") {
		return true
	}
	if body, ok := file.FlatParent(idx); ok && file.FlatType(body) == "interface_body" {
		return true
	}
	return false
}

// workResolveTypeRef lists the FQNs a written type name may denote in the file: an explicit import (possibly of an
// outer class), a fully qualified name, a class of the same package, or a class under an on-demand import.
func workResolveTypeRef(file *scanner.File, pkg, text string) []string {
	text = strings.TrimSpace(strings.TrimSuffix(text, "?"))
	if text == "" {
		return nil
	}
	imp := lintImportsOf(file)
	head, rest, _ := strings.Cut(text, ".")
	if full, ok := imp.bySimple[head]; ok {
		if rest != "" {
			full += "." + rest
		}
		return []string{full}
	}
	var out []string
	if head != "" && head[0] >= 'a' && head[0] <= 'z' {
		out = append(out, text)
	}
	same := text
	if pkg != "" {
		same = pkg + "." + text
	}
	out = append(out, same)
	for _, star := range imp.stars {
		out = append(out, star+"."+text)
	}
	if file.Language == scanner.LangJava {
		out = append(out, "java.lang."+text)
	}
	return out
}

// inheritsFrom reports whether the class reaches fqn through project classes and known library classes.
func (p *workProject) inheritsFrom(c *workClass, fqn string) bool {
	return p.reaches(c.supers, fqn, map[string]bool{c.fqn: true}, 0)
}

func (p *workProject) reaches(supers [][]string, fqn string, seen map[string]bool, depth int) bool {
	if depth > 12 {
		return false
	}
	for _, candidates := range supers {
		for _, cand := range candidates {
			if cand == fqn {
				return true
			}
			if seen[cand] {
				continue
			}
			seen[cand] = true
			if c, ok := p.byFQN[cand]; ok {
				if p.reaches(c.supers, fqn, seen, depth+1) {
					return true
				}
				break
			}
			if lib, ok := workLibrarySupertypes[cand]; ok {
				if p.reaches([][]string{lib}, fqn, seen, depth+1) {
					return true
				}
				break
			}
		}
	}
	return false
}

// workProjectCallsNamed reports whether any project call has the given method name. Lint also checks that the
// method belongs to Configuration.Builder; krit takes any call of that name as the evidence, so a builder it cannot
// trace (one returned by a helper) never produces a report.
func workProjectCallsNamed(files []*scanner.File, name string) bool {
	needle := []byte(name)
	for _, file := range files {
		if !bytes.Contains(file.Content, needle) {
			continue
		}
		found := false
		for _, kind := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, kind, func(idx uint32) {
				if !found {
					if call, ok := lintCallAt(file, idx); ok && call.Name == name {
						found = true
					}
				}
			})
		}
		if found {
			return true
		}
	}
	return false
}

func workNameFinding(r AndroidRule, c *workClass, msg string) scanner.Finding {
	name := lintClassNameNode(c.file, c.idx)
	if name == 0 {
		name = c.idx
	}
	return r.Finding(c.file, c.file.FlatRow(name)+1, c.file.FlatCol(name)+1, msg)
}

// BadConfigurationProviderRule mirrors BadConfigurationProviderIssueDetector.
type BadConfigurationProviderRule struct {
	AndroidRule
}

// Confidence is high: a project class must implement Configuration.Provider while no Application does.
func (r *BadConfigurationProviderRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *BadConfigurationProviderRule) check(ctx *api.Context) {
	if !workMayBePresent(ctx) {
		return
	}
	files := workProjectFiles(ctx)
	p := buildWorkProject(files)
	// Lint checks each module on its own unless checkDependencies is set, so a Configuration.Provider in a library
	// module (a multi-process RemoteWorkerService, say) says nothing about an app module's Application.
	type moduleState struct {
		last        *workClass
		hasProvider bool
		correct     bool
	}
	roots := map[string]string{}
	var order []string
	modules := map[string]*moduleState{}
	for _, c := range p.classes {
		if c.abstract {
			continue
		}
		root := workModuleRoot(c.file.Path, roots)
		m := modules[root]
		if m == nil {
			m = &moduleState{}
			modules[root] = m
			order = append(order, root)
		}
		if m.correct {
			continue
		}
		app := p.inheritsFrom(c, workApplicationClass)
		provider := p.inheritsFrom(c, workConfigProvider)
		if app && provider {
			m.correct = true
			continue
		}
		if app {
			m.last = c
		}
		m.hasProvider = m.hasProvider || provider
	}
	for _, root := range order {
		if m := modules[root]; !m.correct && m.last != nil && m.hasProvider {
			// Lint reports the whole file of the last Application it visited; krit anchors that at its first line.
			ctx.Emit(r.Finding(m.last.file, 1, 1, workBadConfigMessage))
		}
	}
}

// workModuleRoot returns the directory of the nearest build.gradle(.kts) above path, or "" when there is none, so
// sources without build files form one module. cache maps directories already looked up to their root.
func workModuleRoot(path string, cache map[string]string) string {
	var visited []string
	root := ""
	for dir, depth := filepath.Dir(path), 0; depth < 24; dir, depth = filepath.Dir(dir), depth+1 {
		if cached, ok := cache[dir]; ok {
			root = cached
			break
		}
		visited = append(visited, dir)
		if workIsFile(filepath.Join(dir, "build.gradle.kts")) || workIsFile(filepath.Join(dir, "build.gradle")) {
			root = dir
			break
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	for _, dir := range visited {
		cache[dir] = root
	}
	return root
}

func workIsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// SpecifyJobSchedulerIDRangeRule mirrors SpecifyJobSchedulerIdRangeIssueDetector.
type SpecifyJobSchedulerIDRangeRule struct {
	AndroidRule
}

// Confidence is high: a project JobService and the absence of any setJobSchedulerJobIdRange call are both seen.
func (r *SpecifyJobSchedulerIDRangeRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *SpecifyJobSchedulerIDRangeRule) check(ctx *api.Context) {
	files := workProjectFiles(ctx)
	if !workProjectUsesWorkManager(ctx, files) {
		return
	}
	p := buildWorkProject(files)
	for _, c := range p.classes {
		if c.fqn == workSystemJobService || !p.inheritsFrom(c, workJobServiceClass) {
			continue
		}
		if !workProjectCallsNamed(files, "setJobSchedulerJobIdRange") {
			ctx.Emit(workNameFinding(r.AndroidRule, c, workJobIDRangeMessage))
		}
		return
	}
}

// workProjectUsesWorkManager requires positive evidence of WorkManager: a JobService alone says nothing about it, and
// Lint only runs this check when work-runtime is a dependency.
func workProjectUsesWorkManager(ctx *api.Context, files []*scanner.File) bool {
	profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Profile
	if profile.HasAnyDependency(workCoordinates...) {
		return true
	}
	if profile.ProvesDependencyAbsent(workCoordinates...) {
		return false
	}
	for _, file := range files {
		imp := lintImportsOf(file)
		for _, full := range imp.bySimple {
			if strings.HasPrefix(full, "androidx.work.") {
				return true
			}
		}
		for _, star := range imp.stars {
			if star == "androidx.work" || strings.HasPrefix(star, "androidx.work.") {
				return true
			}
		}
	}
	return false
}

// WorkerHasAPublicModifierRule mirrors WorkerHasPublicModifierDetector.
type WorkerHasAPublicModifierRule struct {
	AndroidRule
}

// Confidence is high: the class must reach ListenableWorker and no setWorkerFactory call may exist in the project.
func (r *WorkerHasAPublicModifierRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WorkerHasAPublicModifierRule) check(ctx *api.Context) {
	if !workMayBePresent(ctx) {
		return
	}
	files := workProjectFiles(ctx)
	p := buildWorkProject(files)
	var workers []*workClass
	for _, c := range p.classes {
		if !c.abstract && !c.public && p.inheritsFrom(c, workListenableWorker) {
			workers = append(workers, c)
		}
	}
	if len(workers) == 0 || workProjectCallsNamed(files, "setWorkerFactory") {
		return
	}
	for _, c := range workers {
		ctx.Emit(workNameFinding(r.AndroidRule, c, c.fqn+workPublicWorkerSuffix))
	}
}
