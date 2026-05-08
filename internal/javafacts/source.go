package javafacts

import (
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
)

type SourceIndex struct {
	Files          map[string]*JavaFileFacts
	ClassesByFQN   map[string]JavaClassFact
	ClassesByName  map[string][]JavaClassFact
	PackageClasses map[string]map[string]JavaClassFact
}

type JavaFileFacts struct {
	File            string
	Content         string
	Package         string
	Imports         map[string]string
	StaticImports   map[string]string
	WildcardImports []string
	Classes         map[string]JavaClassFact
}

type JavaClassFact struct {
	Name        string
	FQN         string
	Package     string
	Supertypes  []string
	Annotations []string
	Methods     map[string][]JavaMethodFact
	Fields      map[string]JavaFieldFact
}

type JavaMethodFact struct {
	Name        string
	ReturnType  string
	Annotations []string
	Static      bool
}

type JavaFieldFact struct {
	Name        string
	Type        string
	Annotations []string
	Static      bool
}

var (
	sourceFactsCache sync.Map // *scanner.File -> *JavaFileFacts
	sourceIndexCache sync.Map // string -> *SourceIndex

	javaPackageRe = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_$][A-Za-z0-9_$.]*)\s*;`)
	javaImportRe  = regexp.MustCompile(`(?m)^\s*import\s+(static\s+)?([A-Za-z_$][A-Za-z0-9_$.*]*)\s*;`)
	javaAnnoRe    = regexp.MustCompile(`@([A-Za-z_$][A-Za-z0-9_$.]*)`)
)

func SourceFactsForFile(file *scanner.File) *JavaFileFacts {
	if file == nil || file.Language != scanner.LangJava {
		return nil
	}
	if cached, ok := sourceFactsCache.Load(file); ok {
		return cached.(*JavaFileFacts)
	}
	facts := buildSourceFactsForFile(file)
	sourceFactsCache.Store(file, facts)
	return facts
}

func SourceIndexForFiles(files []*scanner.File) *SourceIndex {
	key := sourceIndexKey(files)
	if key != "" {
		if cached, ok := sourceIndexCache.Load(key); ok {
			return cached.(*SourceIndex)
		}
	}
	idx := &SourceIndex{
		Files:          make(map[string]*JavaFileFacts),
		ClassesByFQN:   make(map[string]JavaClassFact),
		ClassesByName:  make(map[string][]JavaClassFact),
		PackageClasses: make(map[string]map[string]JavaClassFact),
	}
	for _, file := range files {
		facts := SourceFactsForFile(file)
		if facts == nil {
			continue
		}
		idx.Files[file.Path] = facts
		for _, classFact := range facts.Classes {
			idx.ClassesByFQN[classFact.FQN] = classFact
			idx.ClassesByName[classFact.Name] = append(idx.ClassesByName[classFact.Name], classFact)
			if idx.PackageClasses[classFact.Package] == nil {
				idx.PackageClasses[classFact.Package] = make(map[string]JavaClassFact)
			}
			idx.PackageClasses[classFact.Package][classFact.Name] = classFact
		}
	}
	if key != "" {
		sourceIndexCache.Store(key, idx)
	}
	return idx
}

func (f *JavaFileFacts) ResolveType(name string, project *SourceIndex) string {
	name = normalizeJavaTypeName(name)
	if name == "" {
		return ""
	}
	if strings.Contains(name, ".") {
		return name
	}
	if classFact, ok := f.Classes[name]; ok {
		return classFact.FQN
	}
	if project != nil {
		if classFact, ok := project.PackageClasses[f.Package][name]; ok {
			return classFact.FQN
		}
	}
	if imported := f.Imports[name]; imported != "" {
		return imported
	}
	for _, pkg := range f.WildcardImports {
		if project != nil {
			if classFact, ok := project.PackageClasses[pkg][name]; ok {
				return classFact.FQN
			}
		}
		if known := knownJavaType(pkg, name); known != "" {
			return known
		}
	}
	if known := knownJavaSimpleType(name); known != "" {
		return known
	}
	if f.Package != "" {
		return f.Package + "." + name
	}
	return name
}

func (f *JavaFileFacts) ImportsOrMentions(fqn string) bool {
	if f == nil || fqn == "" {
		return false
	}
	simple := simpleJavaName(fqn)
	for _, imported := range f.Imports {
		if imported == fqn {
			return true
		}
	}
	for _, imported := range f.StaticImports {
		if imported == fqn || strings.HasPrefix(imported, fqn+".") {
			return true
		}
	}
	pkg := packagePart(fqn)
	for _, wildcard := range f.WildcardImports {
		if wildcard == pkg && knownJavaType(wildcard, simple) == fqn {
			return true
		}
	}
	return strings.Contains(stringContent(f), fqn)
}

func buildSourceFactsForFile(file *scanner.File) *JavaFileFacts {
	content := string(file.Content)
	facts := &JavaFileFacts{
		File:          file.Path,
		Content:       content,
		Package:       javaPackageFromContent(content),
		Imports:       make(map[string]string),
		StaticImports: make(map[string]string),
		Classes:       make(map[string]JavaClassFact),
	}
	for _, match := range javaImportRe.FindAllStringSubmatch(content, -1) {
		target := strings.TrimSpace(match[2])
		if strings.HasSuffix(target, ".*") {
			pkg := strings.TrimSuffix(target, ".*")
			if match[1] != "" {
				facts.StaticImports["*:"+pkg] = pkg
			} else {
				facts.WildcardImports = append(facts.WildcardImports, pkg)
			}
			continue
		}
		simple := simpleJavaName(target)
		if match[1] != "" {
			facts.StaticImports[simple] = target
		} else {
			facts.Imports[simple] = target
		}
	}
	if file.FlatTree == nil {
		return facts
	}
	file.FlatWalkAllNodes(0, func(node uint32) {
		switch file.FlatType(node) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			addClassFact(file, facts, node)
		}
	})
	file.FlatWalkAllNodes(0, func(node uint32) {
		switch file.FlatType(node) {
		case "method_declaration":
			addMethodFact(file, facts, node)
		case "field_declaration":
			addFieldFacts(file, facts, node)
		}
	})
	return facts
}

func addClassFact(file *scanner.File, facts *JavaFileFacts, node uint32) {
	name := file.FlatChildTextOrEmpty(node, "identifier")
	if name == "" {
		return
	}
	owner := javaOwnerName(file, facts.Package, node)
	fqn := name
	if owner != "" {
		fqn = owner + "." + name
	} else if facts.Package != "" {
		fqn = facts.Package + "." + name
	}
	classFact := JavaClassFact{
		Name:        name,
		FQN:         fqn,
		Package:     facts.Package,
		Supertypes:  javaHeaderSupertypes(file.FlatNodeText(node)),
		Annotations: javaAnnotations(javaDeclarationHeader(file.FlatNodeText(node))),
		Methods:     make(map[string][]JavaMethodFact),
		Fields:      make(map[string]JavaFieldFact),
	}
	facts.Classes[name] = classFact
}

func addMethodFact(file *scanner.File, facts *JavaFileFacts, node uint32) {
	ownerName, ok := enclosingJavaClassName(file, node)
	if !ok {
		return
	}
	classFact, ok := facts.Classes[ownerName]
	if !ok {
		return
	}
	name := file.FlatChildTextOrEmpty(node, "identifier")
	if name == "" {
		return
	}
	method := JavaMethodFact{
		Name:        name,
		ReturnType:  javaMethodReturnType(file.FlatNodeText(node), name),
		Annotations: javaAnnotations(javaDeclarationHeader(file.FlatNodeText(node))),
		Static:      file.FlatHasModifier(node, "static"),
	}
	classFact.Methods[name] = append(classFact.Methods[name], method)
	facts.Classes[ownerName] = classFact
}

func addFieldFacts(file *scanner.File, facts *JavaFileFacts, node uint32) {
	ownerName, ok := enclosingJavaClassName(file, node)
	if !ok {
		return
	}
	classFact, ok := facts.Classes[ownerName]
	if !ok {
		return
	}
	text := file.FlatNodeText(node)
	annotations := javaAnnotations(text)
	static := file.FlatHasModifier(node, "static")
	file.FlatWalkNodes(node, "variable_declarator", func(child uint32) {
		name := file.FlatChildTextOrEmpty(child, "identifier")
		if name == "" {
			return
		}
		typ := javaFieldType(text, name)
		classFact.Fields[name] = JavaFieldFact{Name: name, Type: typ, Annotations: annotations, Static: static}
	})
	facts.Classes[ownerName] = classFact
}

func javaPackageFromContent(content string) string {
	match := javaPackageRe.FindStringSubmatch(content)
	if len(match) == 0 {
		return ""
	}
	return match[1]
}

func javaOwnerName(file *scanner.File, pkg string, node uint32) string {
	var names []string
	for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			if name := file.FlatChildTextOrEmpty(parent, "identifier"); name != "" {
				names = append(names, name)
			}
		}
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	owner := strings.Join(names, ".")
	if owner != "" && pkg != "" {
		return pkg + "." + owner
	}
	if owner != "" {
		return owner
	}
	return ""
}

func enclosingJavaClassName(file *scanner.File, node uint32) (string, bool) {
	for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			name := file.FlatChildTextOrEmpty(parent, "identifier")
			return name, name != ""
		}
	}
	return "", false
}

func javaHeaderSupertypes(text string) []string {
	header := javaDeclarationHeader(text)
	var out []string
	for _, marker := range []string{" extends ", " implements "} {
		idx := strings.Index(header, marker)
		if idx < 0 {
			continue
		}
		rest := header[idx+len(marker):]
		if next := strings.Index(rest, " implements "); next >= 0 {
			rest = rest[:next]
		}
		for _, part := range strings.Split(rest, ",") {
			if typ := normalizeJavaTypeName(part); typ != "" {
				out = append(out, typ)
			}
		}
	}
	return uniqueStrings(out)
}

func javaAnnotations(text string) []string {
	var out []string
	for _, match := range javaAnnoRe.FindAllStringSubmatch(text, -1) {
		out = append(out, match[1])
	}
	return uniqueStrings(out)
}

func javaMethodReturnType(text, name string) string {
	header := javaDeclarationHeader(text)
	idx := strings.Index(header, name+"(")
	if idx < 0 {
		return ""
	}
	before := strings.TrimSpace(header[:idx])
	fields := strings.Fields(before)
	for i := len(fields) - 1; i >= 0; i-- {
		field := fields[i]
		if strings.HasPrefix(field, "@") || javaModifierWords[field] {
			continue
		}
		return normalizeJavaTypeName(field)
	}
	return ""
}

func javaFieldType(text, name string) string {
	header := strings.TrimSpace(strings.SplitN(text, "=", 2)[0])
	header = strings.TrimSuffix(header, ";")
	if idx := strings.Index(header, name); idx >= 0 {
		header = strings.TrimSpace(header[:idx])
	}
	fields := strings.Fields(header)
	for i := len(fields) - 1; i >= 0; i-- {
		field := fields[i]
		if strings.HasPrefix(field, "@") || javaModifierWords[field] {
			continue
		}
		return normalizeJavaTypeName(field)
	}
	return ""
}

func javaDeclarationHeader(text string) string {
	if idx := strings.Index(text, "{"); idx >= 0 {
		text = text[:idx]
	}
	if idx := strings.Index(text, ";"); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}

func normalizeJavaTypeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, ";,")
	value = strings.TrimPrefix(value, "new ")
	if idx := strings.Index(value, "<"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSuffix(value, "[]")
	return strings.TrimSpace(value)
}

func simpleJavaName(fqn string) string {
	if idx := strings.LastIndex(fqn, "."); idx >= 0 {
		return fqn[idx+1:]
	}
	return fqn
}

func packagePart(fqn string) string {
	if idx := strings.LastIndex(fqn, "."); idx >= 0 {
		return fqn[:idx]
	}
	return ""
}

func sourceIndexKey(files []*scanner.File) string {
	var parts []string
	for _, file := range files {
		if file != nil && file.Language == scanner.LangJava {
			hash := fnv.New64a()
			_, _ = hash.Write(file.Content)
			parts = append(parts, file.Path, strconv.Itoa(len(file.Content)), strconv.FormatUint(hash.Sum64(), 16))
		}
	}
	return strings.Join(parts, "\x00")
}

func stringContent(f *JavaFileFacts) string {
	if f == nil {
		return ""
	}
	if f.Content != "" {
		return f.Content
	}
	var b strings.Builder
	for _, imported := range f.Imports {
		b.WriteString(imported)
		b.WriteByte('\n')
	}
	for _, imported := range f.StaticImports {
		b.WriteString(imported)
		b.WriteByte('\n')
	}
	return b.String()
}

func knownJavaSimpleType(name string) string {
	switch name {
	case "String", "Object", "Runnable", "Thread", "Exception", "RuntimeException", "Integer", "Long", "Boolean", "Void", "System", "Runtime":
		return "java.lang." + name
	case "List", "Map", "Set", "Collection", "ArrayList", "HashMap", "HashSet", "Collections":
		return "java.util." + name
	case "WebView", "WebSettings", "JavascriptInterface":
		return "android.webkit." + name
	case "SharedPreferences":
		return "android.content.SharedPreferences"
	case "FragmentManager", "FragmentTransaction":
		return "androidx.fragment.app." + name
	case "FileInputStream", "BufferedInputStream", "InputStream":
		return "java.io." + name
	}
	return ""
}

func knownJavaType(pkg, name string) string {
	known := knownJavaSimpleType(name)
	if known != "" && packagePart(known) == pkg {
		return known
	}
	return ""
}

var javaModifierWords = map[string]bool{
	"public": true, "private": true, "protected": true, "static": true, "final": true,
	"abstract": true, "synchronized": true, "native": true, "strictfp": true,
	"transient": true, "volatile": true, "default": true,
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
