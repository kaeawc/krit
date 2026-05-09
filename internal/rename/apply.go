package rename

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
)

// ApplyResult summarises a successful rename apply pass.
type ApplyResult struct {
	FilesChanged int
	Edits        int
}

// edit describes one byte-range substitution within a single file.
type edit struct {
	StartByte int
	EndByte   int
	Replace   string
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
	return result, nil
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
		current := string(content[e.StartByte:e.EndByte])
		if current != plan.Target.FromName {
			return nil, fmt.Errorf("rename apply: %s byte range %d..%d holds %q, not %q (file changed since indexing?)", file, e.StartByte, e.EndByte, current, plan.Target.FromName)
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
