package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// structureArgs are the arguments for the structure tool.
type structureArgs struct {
	Operation string `json:"operation"`

	// operation=modules/profile
	ProjectRoot string `json:"project_root"`
	Module      string `json:"module"`

	// operation=hotspots/breadth/pkg_drift
	Paths     []string `json:"paths"`
	Threshold int      `json:"threshold"`
	Limit     int      `json:"limit"`
}

func (s *Server) toolStructure(arguments json.RawMessage) ToolResult {
	var args structureArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	op := args.Operation
	if op == "" {
		op = opStructureModules
	}

	switch op {
	case opStructureModules:
		return s.structureModules(args)
	case opStructureProfile:
		return s.structureProfile(args)
	case opStructureHotspots:
		return s.structureHotspots(args)
	case opStructureBreadth:
		return s.structureBreadth(args)
	case opStructurePkgDrift:
		return s.structurePkgDrift(args)
	default:
		return errorResult("unknown operation: " + op + "; valid: " + formatList(structureOperations))
	}
}

// structureModules discovers Gradle modules.
func (s *Server) structureModules(args structureArgs) ToolResult {
	if args.ProjectRoot == "" {
		return errorResult("'project_root' argument is required for operation=modules")
	}

	info, err := os.Stat(args.ProjectRoot)
	if err != nil {
		return errorResult("cannot access project root: " + err.Error())
	}
	if !info.IsDir() {
		return errorResult("project_root must be a directory")
	}

	graph, err := module.DiscoverModules(args.ProjectRoot)
	if err != nil {
		return errorResult("discovering modules: " + err.Error())
	}
	if graph == nil {
		return errorResult("no settings.gradle.kts or settings.gradle found in " + args.ProjectRoot)
	}

	type moduleJSON struct {
		Path        string   `json:"path"`
		Dir         string   `json:"dir"`
		SourceRoots []string `json:"sourceRoots,omitempty"`
		DependsOn   []string `json:"dependsOn,omitempty"`
	}

	if args.Module != "" {
		modPath := args.Module
		if !strings.HasPrefix(modPath, ":") {
			modPath = ":" + modPath
		}
		m, ok := graph.Modules[modPath]
		if !ok {
			return errorResult("module not found: " + args.Module)
		}
		var deps []string
		for _, d := range m.Dependencies {
			deps = append(deps, d.ModulePath)
		}
		return jsonResult(moduleJSON{
			Path:        m.Path,
			Dir:         m.Dir,
			SourceRoots: m.SourceRoots,
			DependsOn:   deps,
		})
	}

	type graphJSON struct {
		RootDir     string       `json:"rootDir"`
		ModuleCount int          `json:"moduleCount"`
		Modules     []moduleJSON `json:"modules"`
	}

	modules := make([]moduleJSON, 0, len(graph.Modules))
	for _, m := range graph.Modules {
		var deps []string
		for _, d := range m.Dependencies {
			deps = append(deps, d.ModulePath)
		}
		modules = append(modules, moduleJSON{
			Path:        m.Path,
			Dir:         m.Dir,
			SourceRoots: m.SourceRoots,
			DependsOn:   deps,
		})
	}
	sort.SliceStable(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })

	return jsonResult(graphJSON{
		RootDir:     graph.RootDir,
		ModuleCount: len(graph.Modules),
		Modules:     modules,
	})
}

// structureProfile detects framework presence (Compose, Room, SQLDelight, etc.)
// from Gradle build files.
func (s *Server) structureProfile(args structureArgs) ToolResult {
	if args.ProjectRoot == "" {
		return errorResult("'project_root' argument is required for operation=profile")
	}

	info, err := os.Stat(args.ProjectRoot)
	if err != nil {
		return errorResult("cannot access project root: " + err.Error())
	}
	if !info.IsDir() {
		return errorResult("project_root must be a directory")
	}

	proj := android.DetectProject([]string{args.ProjectRoot})
	if len(proj.GradlePaths) == 0 {
		return errorResult("no Gradle build files found under " + args.ProjectRoot)
	}

	profile := librarymodel.ProfileFromGradlePaths(proj.GradlePaths)

	return jsonResult(profile)
}

// structureHotspots returns a ranked list of symbols with the highest fan-in.
func (s *Server) structureHotspots(args structureArgs) ToolResult {
	if len(args.Paths) == 0 {
		return errorResult("'paths' argument is required for operation=hotspots")
	}

	index, err := buildIndexForPaths(args.Paths)
	if err != nil {
		return errorResult(err.Error())
	}

	threshold := args.Threshold
	if threshold <= 0 {
		threshold = 10
	}

	fanIn := arch.SymbolFanIn(index)
	hotspots := arch.FilterHotspots(index, fanIn, threshold)

	limit := args.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(hotspots) > limit {
		hotspots = hotspots[:limit]
	}

	type hotspotResult struct {
		Threshold int            `json:"threshold"`
		Total     int            `json:"total"`
		Hotspots  []arch.Hotspot `json:"hotspots"`
	}

	return jsonResult(hotspotResult{
		Threshold: threshold,
		Total:     len(hotspots),
		Hotspots:  hotspots,
	})
}

// structureBreadth returns files importing the most symbols (refactor candidates).
func (s *Server) structureBreadth(args structureArgs) ToolResult {
	if len(args.Paths) == 0 {
		return errorResult("'paths' argument is required for operation=breadth")
	}

	ktFiles, err := scanner.CollectKotlinFiles(args.Paths, nil)
	if err != nil {
		return errorResult("collecting files: " + err.Error())
	}
	if len(ktFiles) == 0 {
		return errorResult("no Kotlin files found in the specified paths")
	}

	files, _ := scanner.ScanFiles(context.Background(), ktFiles, 4)

	threshold := args.Threshold
	if threshold <= 0 {
		threshold = 30
	}

	broad := arch.FindBroadFiles(files, threshold)

	limit := args.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(broad) > limit {
		broad = broad[:limit]
	}

	type breadthResult struct {
		Threshold int              `json:"threshold"`
		Total     int              `json:"total"`
		Files     []arch.BroadFile `json:"files"`
	}

	return jsonResult(breadthResult{
		Threshold: threshold,
		Total:     len(broad),
		Files:     broad,
	})
}

// structurePkgDrift finds files whose Kotlin package doesn't match their directory.
// Source roots come from the discovered Gradle module graph rooted at args.Paths[0],
// so non-canonical layouts (androidMain, commonMain, custom srcDirs) are honored.
func (s *Server) structurePkgDrift(args structureArgs) ToolResult {
	if len(args.Paths) == 0 {
		return errorResult("'paths' argument is required for operation=pkg_drift")
	}

	// Discover modules from the first path so we can resolve real source roots.
	// Fall back to canonical /src/{main,test}/{kotlin,java} if no graph is found.
	graph, _ := module.DiscoverModules(args.Paths[0])

	ktFiles, err := scanner.CollectKotlinFiles(args.Paths, nil)
	if err != nil {
		return errorResult("collecting files: " + err.Error())
	}
	if len(ktFiles) == 0 {
		return errorResult("no Kotlin files found in the specified paths")
	}

	files, _ := scanner.ScanFiles(context.Background(), ktFiles, 4)

	type driftJSON struct {
		File            string `json:"file"`
		ExpectedPackage string `json:"expectedPackage"`
		ActualPackage   string `json:"actualPackage"`
		SourceRoot      string `json:"sourceRoot"`
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 100
	}

	drifts := make([]driftJSON, 0, limit)
	total := 0
	for _, f := range files {
		sourceRoot := findSourceRootForPath(graph, f.Path)
		if sourceRoot == "" {
			continue
		}
		drift := arch.PackageNameDrift(f, sourceRoot)
		if drift == nil {
			continue
		}
		total++
		if len(drifts) >= limit {
			continue
		}
		drifts = append(drifts, driftJSON{
			File:            f.Path,
			ExpectedPackage: drift.Expected,
			ActualPackage:   drift.Declared,
			SourceRoot:      sourceRoot,
		})
	}

	type driftResult struct {
		Total     int         `json:"total"`
		Truncated bool        `json:"truncated,omitempty"`
		Drifts    []driftJSON `json:"drifts"`
	}

	return jsonResult(driftResult{
		Total:     total,
		Truncated: total > len(drifts),
		Drifts:    drifts,
	})
}

// buildIndexForPaths collects Kotlin/Java files under the given paths and
// builds a CodeIndex suitable for arch metric queries.
func buildIndexForPaths(paths []string) (*scanner.CodeIndex, error) {
	ktFiles, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		return nil, err
	}
	if len(ktFiles) == 0 {
		return nil, errors.New("no Kotlin files found in the specified paths")
	}
	files, _ := scanner.ScanFiles(context.Background(), ktFiles, 4)
	javaPaths, _ := scanner.CollectJavaFiles(paths, nil)
	javaFiles, _ := scanner.ScanFiles(context.Background(), javaPaths, 4)
	return scanner.BuildIndex(files, 4, javaFiles...), nil
}

// findSourceRootForPath looks up the longest source root from the discovered
// module graph that prefixes path. Returns "" if no module covers it.
func findSourceRootForPath(graph *module.Graph, path string) string {
	if graph == nil {
		return ""
	}
	best := ""
	for _, m := range graph.Modules {
		for _, root := range m.SourceRoots {
			if strings.HasPrefix(path, root+string(os.PathSeparator)) && len(root) > len(best) {
				best = root
			}
		}
	}
	return best
}
