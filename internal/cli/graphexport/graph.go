package graphexport

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// Run implements `krit graph`.
func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "json", "Output format: json, dot, or mermaid")
	scope := fs.String("scope", "module", "Graph scope: module or package")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "graph: resolving %s: %v\n", root, err)
		return 1
	}

	var output string
	switch *scope {
	case "module":
		output, err = renderGraph(absRoot, *format)
	case "package":
		output, err = renderPackageGraph(absRoot, *format)
	default:
		err = fmt.Errorf("unknown scope %q; use module or package", *scope)
	}
	if err != nil {
		fmt.Fprintf(stderr, "graph: %v\n", err)
		return 1
	}
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintf(stderr, "graph: writing output: %v\n", err)
		return 1
	}
	return 0
}

type graphPayload struct {
	Scope   string       `json:"scope"`
	Nodes   []graphNode  `json:"nodes"`
	Edges   []graphEdge  `json:"edges"`
	RootDir string       `json:"rootDir,omitempty"`
	Modules []moduleInfo `json:"modules,omitempty"`
}

type graphNode struct {
	ID string `json:"id"`
}

type graphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

type moduleInfo struct {
	Path        string   `json:"path"`
	Dir         string   `json:"dir"`
	SourceRoots []string `json:"sourceRoots,omitempty"`
	IsPublished bool     `json:"isPublished,omitempty"`
}

func renderGraph(root, format string) (string, error) {
	graph, err := module.DiscoverModules(root)
	if err != nil {
		return "", fmt.Errorf("discovering modules: %w", err)
	}
	if graph == nil {
		return "", fmt.Errorf("no settings.gradle(.kts) found at %s", root)
	}
	if err := module.ParseAllDependencies(graph); err != nil {
		return "", fmt.Errorf("parsing module dependencies: %w", err)
	}
	payload := modulePayload(graph)
	return renderPayload(payload, format)
}

func modulePayload(graph *module.Graph) graphPayload {
	paths := make([]string, 0, len(graph.Modules))
	for path := range graph.Modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	payload := graphPayload{Scope: "module", RootDir: graph.RootDir}
	for _, path := range paths {
		mod := graph.Modules[path]
		payload.Nodes = append(payload.Nodes, graphNode{ID: path})
		payload.Modules = append(payload.Modules, moduleInfo{
			Path:        mod.Path,
			Dir:         filepath.ToSlash(mod.Dir),
			SourceRoots: toSlashSlice(mod.SourceRoots),
			IsPublished: mod.IsPublished,
		})
		deps := append([]module.Dependency(nil), mod.Dependencies...)
		sort.Slice(deps, func(i, j int) bool {
			if deps[i].ModulePath != deps[j].ModulePath {
				return deps[i].ModulePath < deps[j].ModulePath
			}
			return deps[i].Configuration < deps[j].Configuration
		})
		for _, dep := range deps {
			payload.Edges = append(payload.Edges, graphEdge{From: path, To: dep.ModulePath, Label: dep.Configuration})
		}
	}
	return payload
}

func renderPackageGraph(root, format string) (string, error) {
	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return "", fmt.Errorf("collecting Kotlin files: %w", err)
	}
	files, parseErrs := scanner.ScanFiles(context.Background(), paths, runtime.NumCPU())
	if len(parseErrs) > 0 {
		return "", fmt.Errorf("parsing Kotlin files: %w", parseErrs[0])
	}
	pkgGraph := rules.PackageDependencyGraph(files)
	payload := graphPayload{Scope: "package", RootDir: root}
	for _, node := range pkgGraph.Packages {
		payload.Nodes = append(payload.Nodes, graphNode{ID: node})
	}
	for _, edge := range pkgGraph.Edges {
		payload.Edges = append(payload.Edges, graphEdge{From: edge.From, To: edge.To})
	}
	return renderPayload(payload, format)
}

func renderPayload(payload graphPayload, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	case "dot":
		return renderDOT(payload), nil
	case "mermaid":
		return renderMermaid(payload), nil
	default:
		return "", fmt.Errorf("unknown format %q; use json, dot, or mermaid", format)
	}
}

func renderDOT(payload graphPayload) string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %s {\n", payload.Scope)
	for _, node := range payload.Nodes {
		fmt.Fprintf(&b, "  %q;\n", node.ID)
	}
	for _, edge := range payload.Edges {
		if edge.Label != "" {
			fmt.Fprintf(&b, "  %q -> %q [label=%q];\n", edge.From, edge.To, edge.Label)
		} else {
			fmt.Fprintf(&b, "  %q -> %q;\n", edge.From, edge.To)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func renderMermaid(payload graphPayload) string {
	var b strings.Builder
	b.WriteString("graph TD\n")
	for _, node := range payload.Nodes {
		fmt.Fprintf(&b, "  %s[%q]\n", mermaidID(node.ID), node.ID)
	}
	for _, edge := range payload.Edges {
		if edge.Label != "" {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", mermaidID(edge.From), edge.Label, mermaidID(edge.To))
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", mermaidID(edge.From), mermaidID(edge.To))
		}
	}
	return b.String()
}

func mermaidID(id string) string {
	var b strings.Builder
	b.WriteString("n")
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func toSlashSlice(values []string) []string {
	out := append([]string(nil), values...)
	for i := range out {
		out[i] = filepath.ToSlash(out[i])
	}
	sort.Strings(out)
	return out
}
