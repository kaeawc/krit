package di

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type ExportBinding struct {
	FQN        string             `json:"fqn"`
	Name       string             `json:"name"`
	ModulePath string             `json:"modulePath,omitempty"`
	Scope      string             `json:"scope,omitempty"`
	File       string             `json:"file,omitempty"`
	Line       int                `json:"line,omitempty"`
	Deps       []ExportDependency `json:"deps,omitempty"`
}

type ExportDependency struct {
	ParameterName string `json:"parameterName,omitempty"`
	TypeName      string `json:"typeName"`
	Target        string `json:"target,omitempty"`
	Deferred      bool   `json:"deferred,omitempty"`
}

type ExportGraph struct {
	Bindings []ExportBinding `json:"bindings"`
}

func (g *Graph) ExportJSON(w io.Writer, moduleFilter string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(g.exportGraph(moduleFilter))
}

func (g *Graph) ExportDOT(w io.Writer, moduleFilter string) error {
	bindings := g.sortedBindings(moduleFilter)
	fmt.Fprintln(w, "digraph DI {")
	fmt.Fprintln(w, `  rankdir="LR";`)
	for _, b := range bindings {
		fmt.Fprintf(w, "  %q [label=%q];\n", b.FQN, b.FQN)
	}
	allowed := bindingSet(bindings)
	for _, b := range bindings {
		for _, dep := range b.Dependencies {
			if dep.Target == "" || !allowed[dep.Target] {
				continue
			}
			fmt.Fprintf(w, "  %q -> %q;\n", b.FQN, dep.Target)
		}
	}
	fmt.Fprintln(w, "}")
	return nil
}

func (g *Graph) ExportMermaid(w io.Writer, moduleFilter string) error {
	bindings := g.sortedBindings(moduleFilter)
	fmt.Fprintln(w, "graph LR")
	ids := make(map[string]string, len(bindings))
	for i, b := range bindings {
		id := fmt.Sprintf("B%d", i)
		ids[b.FQN] = id
		fmt.Fprintf(w, "  %s[%q]\n", id, b.FQN)
	}
	for _, b := range bindings {
		for _, dep := range b.Dependencies {
			if targetID := ids[dep.Target]; targetID != "" {
				fmt.Fprintf(w, "  %s --> %s\n", ids[b.FQN], targetID)
			}
		}
	}
	return nil
}

func (g *Graph) exportGraph(moduleFilter string) ExportGraph {
	bindings := g.sortedBindings(moduleFilter)
	out := ExportGraph{Bindings: make([]ExportBinding, 0, len(bindings))}
	for _, b := range bindings {
		item := ExportBinding{
			FQN:        b.FQN,
			Name:       b.Name,
			ModulePath: b.ModulePath,
			File:       b.File,
			Line:       b.Line,
		}
		if b.Scope.Known {
			item.Scope = b.Scope.Name
		}
		for _, dep := range b.Dependencies {
			item.Deps = append(item.Deps, ExportDependency(dep))
		}
		out.Bindings = append(out.Bindings, item)
	}
	return out
}

func (g *Graph) sortedBindings(moduleFilter string) []*Binding {
	if g == nil {
		return nil
	}
	keys := make([]string, 0, len(g.Bindings))
	for key, b := range g.Bindings {
		if b == nil {
			continue
		}
		if moduleFilter != "" && b.ModulePath != moduleFilter {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]*Binding, 0, len(keys))
	for _, key := range keys {
		out = append(out, g.Bindings[key])
	}
	return out
}

func bindingSet(bindings []*Binding) map[string]bool {
	out := make(map[string]bool, len(bindings))
	for _, b := range bindings {
		if b != nil {
			out[b.FQN] = true
		}
	}
	return out
}

func NormalizeExportFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		return "json"
	case "dot":
		return "dot"
	case "mermaid":
		return "mermaid"
	default:
		return ""
	}
}
