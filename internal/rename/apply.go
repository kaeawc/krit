package rename

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// Apply rewrites every reference site in plan to target.ToName and writes
// updated file contents back to disk. Each file is written atomically via
// a sibling temp file plus rename. If any file fails to write, files
// already written are not rolled back — the caller should run apply against
// a clean working tree (typically a git checkout). Use DryRun to compute
// the planned edits without touching disk.
func Apply(plan Plan) (ApplyResult, error) {
	return apply(plan, false)
}

// DryRunApply returns the same result Apply would produce without writing
// anything to disk.
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
		if err := atomicWrite(file, updated); err != nil {
			return result, err
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
		if err := os.Rename(mv.From, mv.To); err != nil {
			return result, fmt.Errorf("rename apply: rename %s -> %s: %w", mv.From, mv.To, err)
		}
		result.Moves = append(result.Moves, mv)
	}
	return result, nil
}

// planFileMoves identifies declaration files whose basename matches the
// rename's FromName and proposes a same-directory rename to ToName + the
// existing extension. Kotlin convention but Java requirement when the
// renamed class is the file's top-level public class.
func planFileMoves(plan Plan) []FileMove {
	if plan.Target.FromName == plan.Target.ToName {
		return nil
	}
	declFiles := declarationFiles(plan)
	out := make([]FileMove, 0)
	seen := make(map[string]bool, len(declFiles))
	for path := range declFiles {
		if seen[path] {
			continue
		}
		seen[path] = true
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		stem := base[:len(base)-len(ext)]
		if stem != plan.Target.FromName {
			continue
		}
		dest := filepath.Join(dir, plan.Target.ToName+ext)
		if dest == path {
			continue
		}
		out = append(out, FileMove{From: path, To: dest})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out
}

func collectIdentifierEdits(plan Plan) map[string][]edit {
	out := make(map[string][]edit)
	for _, ref := range plan.References {
		if ref.StartByte <= 0 && ref.EndByte <= 0 {
			continue
		}
		if ref.EndByte <= ref.StartByte {
			continue
		}
		if ref.Language != scanner.LangKotlin && ref.Language != scanner.LangJava {
			continue
		}
		if _, ok := plan.Contexts[ref.File]; !ok {
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

func applyFileEdits(plan Plan, file string, edits []edit) ([]byte, error) {
	content, err := readFileContent(plan, file)
	if err != nil {
		return nil, err
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].StartByte > edits[j].StartByte })
	for i := 1; i < len(edits); i++ {
		if edits[i].EndByte > edits[i-1].StartByte {
			return nil, fmt.Errorf("rename apply: overlapping edits in %s", file)
		}
	}
	for _, e := range edits {
		if e.StartByte < 0 || e.EndByte > len(content) {
			return nil, fmt.Errorf("rename apply: edit out of bounds in %s (%d..%d, len=%d)", file, e.StartByte, e.EndByte, len(content))
		}
		if e.Expect != "" {
			current := string(content[e.StartByte:e.EndByte])
			if current != e.Expect {
				return nil, fmt.Errorf("rename apply: %s byte range %d..%d holds %q, not %q (file changed since indexing?)", file, e.StartByte, e.EndByte, current, e.Expect)
			}
		}
		content = append(content[:e.StartByte], append([]byte(e.Replace), content[e.EndByte:]...)...)
	}
	return content, nil
}

func readFileContent(plan Plan, path string) ([]byte, error) {
	for _, f := range planFilesFromPlan(plan) {
		if f != nil && f.Path == path && f.Content != nil {
			return append([]byte(nil), f.Content...), nil
		}
	}
	return os.ReadFile(path)
}

// planFilesFromPlan returns the files that were used to build the plan's
// contexts. Stored separately from Contexts to avoid changing the public
// FileContext shape.
func planFilesFromPlan(plan Plan) []*scanner.File {
	if plan.cachedFiles != nil {
		return plan.cachedFiles
	}
	return nil
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

	for _, file := range plan.cachedFiles {
		if file == nil {
			continue
		}
		hr := CollectHeaderRanges(file)

		if declFiles[file.Path] && hr.Package != ([2]int{}) {
			rewriteText := buildPackageReplacement(file, hr.Package, plan.Target.ToPackage())
			if rewriteText != "" {
				editsByFile[file.Path] = append(editsByFile[file.Path], edit{
					StartByte: hr.Package[0],
					EndByte:   hr.Package[1],
					Replace:   rewriteText,
				})
			}
		}

		if rng, ok := hr.Imports[plan.Target.FromFQN]; ok {
			repl := rewriteImportLine(file, rng, plan.Target.FromFQN, plan.Target.ToFQN)
			if repl != "" {
				editsByFile[file.Path] = stripEditsInRange(editsByFile[file.Path], rng)
				editsByFile[file.Path] = append(editsByFile[file.Path], edit{
					StartByte: rng[0],
					EndByte:   rng[1],
					Replace:   repl,
				})
			}
			continue
		}

		// File referenced the symbol but had no explicit import — that is
		// only possible when the file shares the symbol's old package. Now
		// that the symbol is moving away, the file needs an explicit
		// import added.
		if !declFiles[file.Path] && refFiles[file.Path] {
			ctx, ok := plan.Contexts[file.Path]
			if !ok {
				continue
			}
			if ctx.Package != plan.Target.FromPackage() {
				continue
			}
			ins := buildImportInsertion(file, hr, plan.Target.ToFQN)
			if ins == nil {
				continue
			}
			editsByFile[file.Path] = append(editsByFile[file.Path], *ins)
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
// line at the appropriate position in file. Insertion point precedence:
// before the first existing import; else after the package declaration;
// else at the start of the file. Returns nil when no anchor exists.
func buildImportInsertion(file *scanner.File, hr HeaderRanges, toFQN string) *edit {
	semi := ""
	if file.Language == scanner.LangJava {
		semi = ";"
	}
	if first := earliestRange(hr.Imports, hr.Wildcards); first != nil {
		return &edit{
			StartByte: first[0],
			EndByte:   first[0],
			Replace:   "import " + toFQN + semi + "\n",
		}
	}
	if hr.Package != ([2]int{}) {
		// Insert immediately after the package line; clamped Package[1] is
		// the byte before the newline. We add a leading "\n\nimport ..."
		// so the new import is separated from the package declaration.
		return &edit{
			StartByte: hr.Package[1],
			EndByte:   hr.Package[1],
			Replace:   "\n\nimport " + toFQN + semi,
		}
	}
	return &edit{
		StartByte: 0,
		EndByte:   0,
		Replace:   "import " + toFQN + semi + "\n\n",
	}
}

func earliestRange(maps ...map[string][2]int) *[2]int {
	var best *[2]int
	for _, m := range maps {
		for _, r := range m {
			if r[0] == 0 && r[1] == 0 {
				continue
			}
			rr := r
			if best == nil || rr[0] < best[0] {
				best = &rr
			}
		}
	}
	return best
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

func rewriteImportLine(file *scanner.File, rng [2]int, fromFQN, toFQN string) string {
	if file.Content == nil || rng[0] < 0 || rng[1] > len(file.Content) || rng[0] >= rng[1] {
		return ""
	}
	original := strings.TrimSpace(string(file.Content[rng[0]:rng[1]]))
	hasSemi := strings.HasSuffix(original, ";")
	body := strings.TrimSuffix(original, ";")
	body = strings.TrimSpace(body)
	body = strings.TrimPrefix(body, "import")
	body = strings.TrimSpace(body)

	prefix := "import "
	suffix := ""
	if file.Language == scanner.LangJava {
		if strings.HasPrefix(body, "static ") {
			prefix = "import static "
		}
	} else {
		// Kotlin: preserve `as <alias>` if present.
		if i := strings.Index(body, " as "); i >= 0 {
			suffix = body[i:]
		}
	}

	out := prefix + toFQN + suffix
	if file.Language == scanner.LangJava || hasSemi {
		out += ";"
	}
	_ = fromFQN
	return out
}

func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".krit-rename-*")
	if err != nil {
		return fmt.Errorf("rename apply: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("rename apply: write %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("rename apply: sync %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename apply: close %s: %w", tmpPath, err)
	}
	info, err := os.Stat(path)
	if err == nil {
		_ = os.Chmod(tmpPath, info.Mode())
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename apply: rename %s: %w", path, err)
	}
	return nil
}
