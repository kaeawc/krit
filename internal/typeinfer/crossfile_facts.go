package typeinfer

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// CrossFileFacts summarizes what one file contributes to the resolution of
// other files: the declarations the merged resolver serves to them (classes
// with their supertypes, modifiers and members, sealed and enum entries,
// type aliases, functions and extensions other files can call, suspend
// functions).
type CrossFileFacts struct {
	// ByName maps each name a lookup for one of those declarations goes by,
	// simple and fully qualified, to a hash of what the lookup sees. A
	// supertype's name also covers the file's subtypes of it: a new subclass
	// changes what a sealed or abstract supertype resolves to for files that
	// only name the supertype. Source positions, the file's path, its
	// imports and file-local declarations are left out, so a body-only edit
	// keeps every hash.
	ByName map[string]string
	// Hash covers ByName: equal hashes mean no lookup changed.
	Hash string
}

// ChangedNames returns the names whose lookups differ between a and b,
// sorted: added, removed, or seeing different declarations.
func ChangedNames(a, b map[string]string) []string {
	var out []string
	for n, h := range a {
		if b[n] != h {
			out = append(out, n)
		}
	}
	for n := range b {
		if _, ok := a[n]; !ok {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

type crossFileClass struct {
	Name             string       `json:"n"`
	FQN              string       `json:"f"`
	Kind             string       `json:"k"`
	Supertypes       []string     `json:"s"`
	DirectSupertypes []string     `json:"d"`
	Flags            [6]bool      `json:"b"`
	Members          []MemberInfo `json:"m"`
}

type crossFileExtension struct {
	Receiver string        `json:"r"`
	Name     string        `json:"n"`
	Return   *ResolvedType `json:"t"`
}

// CrossFileFacts returns the facts for info; the zero value for nil.
func (info *FileTypeInfo) CrossFileFacts() CrossFileFacts {
	if info == nil {
		return CrossFileFacts{}
	}
	f := crossFileFrags{}
	f.addClasses(info.Classes)
	for name, subs := range info.SealedSubs {
		f.add(name, "sealed", subs)
	}
	for name, entries := range info.EnumEntries {
		f.add(name, "enum", entries)
	}
	for name, target := range info.TypeAliases {
		f.add(name, "alias", target)
	}
	f.addFunctions(info)
	return f.facts()
}

// crossFileFrags collects, per lookup name, an encoding of each declaration
// the lookup sees.
type crossFileFrags map[string][]string

func (f crossFileFrags) add(name, kind string, v any) {
	if name == "" {
		return
	}
	data, err := json.Marshal(v) // map keys are written sorted
	if err != nil {
		data = []byte("unencodable:" + err.Error())
	}
	frag := kind + ":" + string(data)
	f[name] = append(f[name], frag)
	if i := strings.LastIndexByte(name, '.'); i >= 0 && i+1 < len(name) {
		f[name[i+1:]] = append(f[name[i+1:]], frag)
	}
}

func (f crossFileFrags) addClasses(classes []*ClassInfo) {
	for _, ci := range classes {
		if ci == nil {
			continue
		}
		class := crossFileClass{
			Name: ci.Name, FQN: ci.FQN, Kind: ci.Kind,
			Supertypes: ci.Supertypes, DirectSupertypes: ci.DirectSupertypes,
			Flags:   [6]bool{ci.IsSealed, ci.IsData, ci.IsInner, ci.IsAbstract, ci.IsOpen, ci.IsNested},
			Members: ci.Members,
		}
		name := ci.FQN
		if name == "" {
			name = ci.Name
		}
		f.add(name, "class", class)
		for _, s := range append(append([]string(nil), ci.Supertypes...), ci.DirectSupertypes...) {
			f.add(s, "subtype", ci.FQN+"|"+ci.Name)
		}
	}
}

// addFunctions records the functions, extensions and suspend functions
// other files can call; a file-local one is only callable from its file.
func (f crossFileFrags) addFunctions(info *FileTypeInfo) {
	for name, ret := range info.Functions {
		f.add(name, "fun", ret)
	}
	for key, decls := range info.FunctionDecls {
		for _, d := range decls {
			if !d.FileLocal {
				f.add(key, "decl", d.Type)
			}
		}
	}
	for _, ext := range info.Extensions {
		if ext != nil && !ext.FileLocal {
			f.add(ext.Name, "ext", crossFileExtension{Receiver: ext.ReceiverType, Name: ext.Name, Return: ext.ReturnType})
		}
	}
	for _, name := range info.SuspendFunctions {
		f.add(name, "suspend", true)
	}
}

func (f crossFileFrags) facts() CrossFileFacts {
	out := CrossFileFacts{ByName: make(map[string]string, len(f))}
	names := make([]string, 0, len(f))
	for name, frags := range f {
		sort.Strings(frags)
		out.ByName[name] = hashutil.HashHex([]byte(strings.Join(frags, "\x00")))[:16]
		names = append(names, name)
	}
	sort.Strings(names)
	var all strings.Builder
	for _, n := range names {
		all.WriteString(n)
		all.WriteByte(0)
		all.WriteString(out.ByName[n])
		all.WriteByte(0)
	}
	out.Hash = hashutil.HashHex([]byte(all.String()))
	return out
}

// FileTypeInfoOf returns the per-file type information the resolver merged
// for path, or nil when path was not indexed.
func (r *defaultResolver) FileTypeInfoOf(path string) *FileTypeInfo {
	if r == nil {
		return nil
	}
	return r.fileInfos[path]
}

// NamesDependingOn returns the lookup names of info's declarations whose
// types mention a name in names, simple or qualified: classes with such a
// supertype, aliases to such a type, and functions and extensions returning
// it or (for extensions) receiving it. When names' declarations change, what
// lookups of these resolve to can change too, so invalidation that follows
// type dependencies through a file expands only these, not every name the
// file declares (a member's simple name like "load" is referenced almost
// everywhere).
func (info *FileTypeInfo) NamesDependingOn(names map[string]bool) []string {
	if info == nil || len(names) == 0 {
		return nil
	}
	d := dependents{names: names}
	for _, ci := range info.Classes {
		if ci != nil && d.anyMentioned(ci.Supertypes, ci.DirectSupertypes) {
			d.add(ci.FQN)
			d.add(ci.Name)
		}
	}
	for name, target := range info.TypeAliases {
		if d.typeMentioned(target) {
			d.add(name)
		}
	}
	for name, ret := range info.Functions {
		if d.typeMentioned(ret) {
			d.add(name)
		}
	}
	for _, ext := range info.Extensions {
		if ext != nil && !ext.FileLocal && (d.mentioned(ext.ReceiverType) || d.typeMentioned(ext.ReturnType)) {
			d.add(ext.Name)
		}
	}
	sort.Strings(d.out)
	return slices.Compact(d.out)
}

// dependents collects NamesDependingOn's result.
type dependents struct {
	names map[string]bool
	out   []string
}

// mentioned reports whether n, or its simple name, is in d.names.
func (d *dependents) mentioned(n string) bool {
	if n == "" {
		return false
	}
	if d.names[n] {
		return true
	}
	i := strings.LastIndexByte(n, '.')
	return i >= 0 && d.names[n[i+1:]]
}

func (d *dependents) typeMentioned(t *ResolvedType) bool {
	return t != nil && (d.mentioned(t.Name) || d.mentioned(t.FQN))
}

func (d *dependents) anyMentioned(lists ...[]string) bool {
	for _, list := range lists {
		for _, n := range list {
			if d.mentioned(n) {
				return true
			}
		}
	}
	return false
}

// add records n and its simple name.
func (d *dependents) add(n string) {
	if n == "" {
		return
	}
	d.out = append(d.out, n)
	if i := strings.LastIndexByte(n, '.'); i >= 0 && i+1 < len(n) {
		d.out = append(d.out, n[i+1:])
	}
}
