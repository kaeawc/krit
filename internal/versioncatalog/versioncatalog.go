// Package versioncatalog parses Gradle version catalogs.
package versioncatalog

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Position is the 1-based line and column of an entry's alias key. For the
// sub-table form ([libraries.core]) it is the table header's position.
type Position struct{ Line, Column int }

type Version struct {
	Ref       string
	Require   string
	Strictly  string
	Prefer    string
	Reject    []string
	RejectAll bool
	Plain     bool
}

// Effective returns Prefer, else Strictly, else Require.
func (v Version) Effective() string {
	if v.Prefer != "" {
		return v.Prefer
	}
	if v.Strictly != "" {
		return v.Strictly
	}
	return v.Require
}

type VersionEntry struct {
	Alias   string
	Pos     Position
	Version Version
}

type Library struct {
	Alias    string
	Pos      Position
	Group    string
	Name     string
	Version  Version
	Resolved string
}

// Module returns "group:name".
func (l Library) Module() string { return l.Group + ":" + l.Name }

type Plugin struct {
	Alias    string
	Pos      Position
	ID       string
	Version  Version
	Resolved string
}

type Bundle struct {
	Alias     string
	Pos       Position
	Libraries []string
}

type Catalog struct {
	Versions  []VersionEntry
	Libraries []Library
	Plugins   []Plugin
	Bundles   []Bundle
}

// Parse validates TOML, then extracts supported Gradle catalog entries.
func Parse(src []byte) (*Catalog, error) {
	var document map[string]any
	if err := toml.Unmarshal(src, &document); err != nil {
		return nil, fmt.Errorf("parse version catalog: %w", err)
	}
	positions, err := entryPositions(src)
	if err != nil {
		return nil, fmt.Errorf("locate version catalog entries: %w", err)
	}

	cat := &Catalog{}
	for alias, value := range section(document, "versions") {
		if version, ok := parseVersion(value); ok {
			cat.Versions = append(cat.Versions, VersionEntry{Alias: alias, Pos: positions[path("versions", alias)], Version: version})
		}
	}
	versions := make(map[string]string, len(cat.Versions))
	for _, entry := range cat.Versions {
		versions[entry.Alias] = entry.Version.Effective()
	}
	for alias, value := range section(document, "libraries") {
		if library, ok := parseLibrary(value); ok {
			library.Alias = alias
			library.Pos = positions[path("libraries", alias)]
			library.Resolved = resolve(library.Version, versions)
			cat.Libraries = append(cat.Libraries, library)
		}
	}
	for alias, value := range section(document, "plugins") {
		if plugin, ok := parsePlugin(value); ok {
			plugin.Alias = alias
			plugin.Pos = positions[path("plugins", alias)]
			plugin.Resolved = resolve(plugin.Version, versions)
			cat.Plugins = append(cat.Plugins, plugin)
		}
	}
	for alias, value := range section(document, "bundles") {
		if libraries, ok := stringArray(value); ok {
			cat.Bundles = append(cat.Bundles, Bundle{Alias: alias, Pos: positions[path("bundles", alias)], Libraries: libraries})
		}
	}
	sort.Slice(cat.Versions, func(i, j int) bool { return before(cat.Versions[i].Pos, cat.Versions[j].Pos) })
	sort.Slice(cat.Libraries, func(i, j int) bool { return before(cat.Libraries[i].Pos, cat.Libraries[j].Pos) })
	sort.Slice(cat.Plugins, func(i, j int) bool { return before(cat.Plugins[i].Pos, cat.Plugins[j].Pos) })
	sort.Slice(cat.Bundles, func(i, j int) bool { return before(cat.Bundles[i].Pos, cat.Bundles[j].Pos) })
	return cat, nil
}

// ParseFile reads and parses a Gradle version catalog.
func ParseFile(filePath string) (*Catalog, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read version catalog: %w", err)
	}
	return Parse(src)
}

func section(document map[string]any, name string) map[string]any {
	entries, _ := document[name].(map[string]any)
	return entries
}

func parseVersion(value any) (Version, bool) {
	if plain, ok := value.(string); ok {
		return Version{Require: plain, Plain: true}, true
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Version{}, false
	}
	var v Version
	seen := false
	for name, value := range fields {
		switch name {
		case "ref", "require", "strictly", "prefer":
			s, ok := value.(string)
			if !ok {
				return Version{}, false
			}
			seen = true
			switch name {
			case "ref":
				v.Ref = s
			case "require":
				v.Require = s
			case "strictly":
				v.Strictly = s
			case "prefer":
				v.Prefer = s
			}
		case "reject":
			var ok bool
			v.Reject, ok = stringArray(value)
			if !ok {
				return Version{}, false
			}
			seen = true
		case "rejectAll":
			var ok bool
			v.RejectAll, ok = value.(bool)
			if !ok {
				return Version{}, false
			}
			seen = true
		}
	}
	return v, seen
}

func parseLibrary(value any) (Library, bool) {
	if s, ok := value.(string); ok {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return Library{}, false
		}
		library := Library{Group: parts[0], Name: parts[1]}
		if len(parts) == 3 {
			library.Version = Version{Require: parts[2], Plain: true}
		}
		return library, true
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Library{}, false
	}
	var library Library
	if module, exists := fields["module"]; exists {
		s, ok := module.(string)
		if !ok {
			return Library{}, false
		}
		parts := strings.SplitN(s, ":", 3)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return Library{}, false
		}
		library.Group, library.Name = parts[0], parts[1]
	} else {
		group, groupOK := fields["group"].(string)
		name, nameOK := fields["name"].(string)
		if !groupOK || !nameOK || group == "" || name == "" {
			return Library{}, false
		}
		library.Group, library.Name = group, name
	}
	if value, exists := fields["version"]; exists {
		library.Version, ok = parseVersion(value)
		if !ok {
			return Library{}, false
		}
	}
	return library, true
}

func parsePlugin(value any) (Plugin, bool) {
	if s, ok := value.(string); ok {
		parts := strings.SplitN(s, ":", 2)
		if parts[0] == "" {
			return Plugin{}, false
		}
		plugin := Plugin{ID: parts[0]}
		if len(parts) == 2 {
			plugin.Version = Version{Require: parts[1], Plain: true}
		}
		return plugin, true
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Plugin{}, false
	}
	id, ok := fields["id"].(string)
	if !ok || id == "" {
		return Plugin{}, false
	}
	plugin := Plugin{ID: id}
	if value, exists := fields["version"]; exists {
		plugin.Version, ok = parseVersion(value)
		if !ok {
			return Plugin{}, false
		}
	}
	return plugin, true
}

func stringArray(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	return result, true
}

func resolve(version Version, versions map[string]string) string {
	if version.Ref != "" {
		return versions[version.Ref]
	}
	return version.Effective()
}

func before(a, b Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Column < b.Column
}

func path(parts ...string) string { return strings.Join(parts, "\x00") }

func entryPositions(src []byte) (map[string]Position, error) {
	positions := make(map[string]Position)
	var parser unstable.Parser
	parser.Reset(src)
	var table []string
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table, unstable.ArrayTable:
			table = nil
			keys := expression.Key()
			if !keys.Next() {
				continue
			}
			first := keys.Node().Raw.Offset
			lineStart := bytes.LastIndexByte(src[:first], '\n') + 1
			bracket := bytes.IndexByte(src[lineStart:first], '[')
			pos := parser.Shape(unstable.Range{Offset: uint32(lineStart + bracket), Length: 1}).Start
			for {
				table = append(table, string(keys.Node().Data))
				remember(positions, table, Position{Line: pos.Line, Column: pos.Column})
				if !keys.Next() {
					break
				}
			}
		case unstable.KeyValue:
			recordKeyValue(&parser, positions, table, expression)
		}
	}
	return positions, parser.Error()
}

func recordKeyValue(parser *unstable.Parser, positions map[string]Position, prefix []string, node *unstable.Node) {
	parts := append([]string(nil), prefix...)
	keys := node.Key()
	for keys.Next() {
		key := keys.Node()
		parts = append(parts, string(key.Data))
		pos := parser.Shape(key.Raw).Start
		remember(positions, parts, Position{Line: pos.Line, Column: pos.Column})
	}
	if value := node.Value(); value.Kind == unstable.InlineTable {
		children := value.Children()
		for children.Next() {
			if child := children.Node(); child.Kind == unstable.KeyValue {
				recordKeyValue(parser, positions, parts, child)
			}
		}
	}
}

func remember(positions map[string]Position, parts []string, pos Position) {
	key := path(parts...)
	if _, exists := positions[key]; !exists {
		positions[key] = pos
	}
}
