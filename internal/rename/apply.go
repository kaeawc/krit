package rename

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/scanner"
)

// ApplyResult summarises a successful rename apply pass.
type ApplyResult struct {
	FilesChanged int
	Edits        int
	Moves        []FileMove
}

// FileMove describes a renamed source file. The Old path no longer exists
// after Apply returns; the From/To-relative directory is unchanged.
type FileMove struct {
	From string
	To   string
}

// edit describes one byte-range substitution within a single file.
type edit struct {
	StartByte int
	EndByte   int
	Replace   string
	// Expect, when non-empty, must equal the existing content at the byte
	// range. Identifier edits use this to detect stale indexes; full-line
	// edits (package, import) leave it empty and trust the AST node range.
	Expect string
}

// Apply writes the rename to disk. Each touched file is rewritten
// atomically. On partial failure, files already written are not rolled
// back — run against a clean working tree (typically a git checkout).
func Apply(plan Plan) (ApplyResult, error) {
	return apply(plan, false)
}

// DryRunApply returns the result Apply would produce without writing.
func DryRunApply(plan Plan) (ApplyResult, error) {
	return apply(plan, true)
}

func apply(plan Plan, dry bool) (ApplyResult, error) {
	if plan.Target.ToName == "" || plan.Target.FromName == "" {
		return ApplyResult{}, fmt.Errorf("rename apply: incomplete target")
	}

	editsByFile := collectIdentifierEdits(plan)
	mergePackageAndImportEdits(plan, editsByFile)

	var result ApplyResult
	files := make([]string, 0, len(editsByFile))
	for f := range editsByFile {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		fileEdits := editsByFile[file]
		if len(fileEdits) == 0 {
			continue
		}
		updated, err := applyFileEdits(plan, file, fileEdits)
		if err != nil {
			return result, err
		}
		if dry {
			result.FilesChanged++
			result.Edits += len(fileEdits)
			continue
		}
		if err := fsutil.WriteFileAtomic(file, updated, fileMode(file)); err != nil {
			return result, fmt.Errorf("rename apply: %w", err)
		}
		result.FilesChanged++
		result.Edits += len(fileEdits)
	}

	moves := planFileMoves(plan)
	for _, mv := range moves {
		if dry {
			result.Moves = append(result.Moves, mv)
			continue
		}
		if _, err := os.Stat(mv.To); err == nil {
			return result, fmt.Errorf("rename apply: destination %s already exists", mv.To)
		}
		if err := os.MkdirAll(filepath.Dir(mv.To), 0o755); err != nil {
			return result, fmt.Errorf("rename apply: mkdir %s: %w", filepath.Dir(mv.To), err)
		}
		if err := os.Rename(mv.From, mv.To); err != nil {
			return result, fmt.Errorf("rename apply: rename %s -> %s: %w", mv.From, mv.To, err)
		}
		result.Moves = append(result.Moves, mv)
	}
	return result, nil
}

// planFileMoves identifies declaration files that need to be renamed,
// moved to a different package directory, or both. The basename gets
// updated when the file's stem matches Target.FromName; the directory
// gets updated when the file's parent path mirrors Target.FromPackage()
// and the rename changes packages.
func planFileMoves(plan Plan) []FileMove {
	declFiles := declarationFiles(plan)
	out := make([]FileMove, 0)
	seen := make(map[string]bool, len(declFiles))
	for path := range declFiles {
		if seen[path] {
			continue
		}
		seen[path] = true
		dest := plannedDestination(path, plan.Target)
		if dest == "" || dest == path {
			continue
		}
		out = append(out, FileMove{From: path, To: dest})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out
}

// plannedDestination computes the new path for a declaration file given
// the target rename. Returns the original path unchanged if neither the
// basename nor the package directory should move.
func plannedDestination(path string, target Target) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]

	newStem := stem
	if stem == target.FromName && target.FromName != target.ToName {
		newStem = target.ToName
	}

	newDir := dir
	if target.PackageChanged() {
		if mapped, ok := remapPackageDir(dir, target.FromPackage(), target.ToPackage()); ok {
			newDir = mapped
		}
	}

	if newDir == dir && newStem == stem {
		return path
	}
	return filepath.Join(newDir, newStem+ext)
}

// remapPackageDir replaces a trailing oldPackage path within dir with
// newPackage. Returns false if dir does not end with the expected
// package path, in which case the caller should leave the directory as
// is (the file may be in a flat layout that doesn't mirror packages).
func remapPackageDir(dir, oldPackage, newPackage string) (string, bool) {
	if oldPackage == "" {
		return dir, false
	}
	oldPath := strings.ReplaceAll(oldPackage, ".", string(filepath.Separator))
	cleanDir := filepath.Clean(dir)
	if cleanDir == oldPath {
		return strings.ReplaceAll(newPackage, ".", string(filepath.Separator)), true
	}
	suffix := string(filepath.Separator) + oldPath
	if !strings.HasSuffix(cleanDir, suffix) {
		return dir, false
	}
	prefix := cleanDir[:len(cleanDir)-len(suffix)]
	newPath := strings.ReplaceAll(newPackage, ".", string(filepath.Separator))
	return filepath.Join(prefix, newPath), true
}

func collectIdentifierEdits(plan Plan) map[string][]edit {
	out := make(map[string][]edit)
	for _, ref := range plan.References {
		if ref.EndByte <= ref.StartByte || ref.StartByte < 0 {
			continue
		}
		if ref.Language != scanner.LangKotlin && ref.Language != scanner.LangJava {
			continue
		}
		if _, ok := plan.contexts[ref.File]; !ok {
			continue
		}
		out[ref.File] = append(out[ref.File], edit{
			StartByte: ref.StartByte,
			EndByte:   ref.EndByte,
			Replace:   plan.Target.ToName,
			Expect:    plan.Target.FromName,
		})
	}
	return out
}

// applyFileEdits returns content with edits spliced in. Edits are sorted
// in ascending byte order and the result is built in a single pass to
// avoid the O(N*E) reslicing cost of doing each edit independently.
func applyFileEdits(plan Plan, file string, edits []edit) ([]byte, error) {
	content, err := readFileContent(plan, file)
	if err != nil {
		return nil, err
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].StartByte < edits[j].StartByte })
	for i := 1; i < len(edits); i++ {
		if edits[i].StartByte < edits[i-1].EndByte {
			return nil, fmt.Errorf("rename apply: overlapping edits in %s", file)
		}
	}

	totalLen := len(content)
	for _, e := range edits {
		if e.StartByte < 0 || e.EndByte > len(content) {
			return nil, fmt.Errorf("rename apply: edit out of bounds in %s (%d..%d, len=%d)", file, e.StartByte, e.EndByte, len(content))
		}
		if e.Expect != "" && string(content[e.StartByte:e.EndByte]) != e.Expect {
			return nil, fmt.Errorf("rename apply: %s byte range %d..%d holds %q, not %q (file changed since indexing?)", file, e.StartByte, e.EndByte, content[e.StartByte:e.EndByte], e.Expect)
		}
		totalLen += len(e.Replace) - (e.EndByte - e.StartByte)
	}

	out := make([]byte, 0, totalLen)
	cursor := 0
	for _, e := range edits {
		out = append(out, content[cursor:e.StartByte]...)
		out = append(out, e.Replace...)
		cursor = e.EndByte
	}
	out = append(out, content[cursor:]...)
	return out, nil
}

func readFileContent(plan Plan, path string) ([]byte, error) {
	if f, ok := plan.filesByPath[path]; ok && f != nil && f.Content != nil {
		return append([]byte(nil), f.Content...), nil
	}
	return os.ReadFile(path)
}

// mergePackageAndImportEdits adds edits for files that need their package
// declaration or import statements rewritten. Triggered when the rename
// crosses package boundaries; same-package renames need no header edits
// because the simple-name rewrite from collectIdentifierEdits already
// updates explicit import lines in place.
func mergePackageAndImportEdits(plan Plan, editsByFile map[string][]edit) {
	if !plan.Target.PackageChanged() {
		return
	}
	declFiles := declarationFiles(plan)
	refFiles := referenceFiles(plan)

	for path, ctx := range plan.contexts {
		file := plan.filesByPath[path]
		if file == nil {
			continue
		}

		if declFiles[path] && ctx.PackageRange != ([2]int{}) {
			if repl := buildPackageReplacement(file, ctx.PackageRange, plan.Target.ToPackage()); repl != "" {
				editsByFile[path] = append(editsByFile[path], edit{
					StartByte: ctx.PackageRange[0],
					EndByte:   ctx.PackageRange[1],
					Replace:   repl,
				})
			}
		}

		if rng, ok := ctx.findImportByFQN(plan.Target.FromFQN); ok {
			if repl := rewriteImportLine(file, rng, plan.Target.ToFQN); repl != "" {
				editsByFile[path] = stripEditsInRange(editsByFile[path], rng)
				editsByFile[path] = append(editsByFile[path], edit{
					StartByte: rng[0],
					EndByte:   rng[1],
					Replace:   repl,
				})
			}
			continue
		}

		// Same-package reference without an explicit import: the file's
		// implicit binding goes away when the symbol moves out, so insert
		// an explicit import.
		if !declFiles[path] && refFiles[path] && ctx.Package == plan.Target.FromPackage() {
			editsByFile[path] = append(editsByFile[path], buildImportInsertion(file, ctx, plan.Target.ToFQN))
		}
	}
}

func referenceFiles(plan Plan) map[string]bool {
	out := make(map[string]bool, len(plan.References))
	for _, r := range plan.References {
		out[r.File] = true
	}
	return out
}

// buildImportInsertion returns an edit that inserts an `import <toFQN>`
// line in file. Insertion point precedence: before the first existing
// import; else after the package declaration; else at the start of the file.
func buildImportInsertion(file *scanner.File, ctx fileContext, toFQN string) edit {
	semi := ""
	if file.Language == scanner.LangJava {
		semi = ";"
	}
	pos, beforeImport := ctx.firstImportAnchor()
	if beforeImport {
		return edit{StartByte: pos, EndByte: pos, Replace: "import " + toFQN + semi + "\n"}
	}
	if ctx.PackageRange != ([2]int{}) {
		// Anchor sits at the byte before the newline that ends the package
		// line, so a leading "\n\n" gives a blank line of separation.
		return edit{StartByte: pos, EndByte: pos, Replace: "\n\nimport " + toFQN + semi}
	}
	return edit{StartByte: 0, EndByte: 0, Replace: "import " + toFQN + semi + "\n\n"}
}

func stripEditsInRange(edits []edit, rng [2]int) []edit {
	out := edits[:0]
	for _, e := range edits {
		if e.StartByte >= rng[0] && e.EndByte <= rng[1] {
			continue
		}
		out = append(out, e)
	}
	return out
}

func declarationFiles(plan Plan) map[string]bool {
	out := make(map[string]bool, len(plan.Declarations))
	for _, d := range plan.Declarations {
		out[d.File] = true
	}
	return out
}

// buildPackageReplacement returns the replacement text for the file's
// package declaration line, preserving the trailing semicolon for Java.
func buildPackageReplacement(file *scanner.File, rng [2]int, toPackage string) string {
	if file.Content == nil || rng[0] < 0 || rng[1] > len(file.Content) || rng[0] >= rng[1] {
		return ""
	}
	original := string(file.Content[rng[0]:rng[1]])
	hasSemi := strings.HasSuffix(strings.TrimSpace(original), ";")
	out := "package " + toPackage
	if file.Language == scanner.LangJava || hasSemi {
		out += ";"
	}
	return out
}

func rewriteImportLine(file *scanner.File, rng [2]int, toFQN string) string {
	if file.Content == nil || rng[0] < 0 || rng[1] > len(file.Content) || rng[0] >= rng[1] {
		return ""
	}
	original := strings.TrimSpace(string(file.Content[rng[0]:rng[1]]))
	hasSemi := strings.HasSuffix(original, ";")
	body := trimImportLine(original)

	prefix := "import "
	suffix := ""
	switch file.Language {
	case scanner.LangJava:
		if strings.HasPrefix(body, "static ") {
			prefix = "import static "
		}
	default:
		if i := strings.Index(body, " as "); i >= 0 {
			suffix = body[i:]
		}
	}

	out := prefix + toFQN + suffix
	if file.Language == scanner.LangJava || hasSemi {
		out += ";"
	}
	return out
}

func fileMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}
