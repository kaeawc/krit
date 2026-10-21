package rules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RoomExportSchemaDisabledRule detects Room @Database(exportSchema = false)
// declarations. Disabling schema export loses migration history.
type RoomExportSchemaDisabledRule struct {
	FlatDispatchBase
	BaseRule
}

// RoomFallbackToDestructiveMigrationRule detects calls to
// `fallbackToDestructiveMigration` on a Room database builder outside debug
// source sets. Falling back to destructive migration silently drops user data
// on a schema version bump.
type RoomFallbackToDestructiveMigrationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule.
// Detection matches on annotation names without confirming the annotated
// class is androidx.room.Database.
func (r *RoomExportSchemaDisabledRule) Confidence() float64 { return api.ConfidenceMedium }

func roomDatabaseExportSchemaDisabledFlat(file *scanner.File, idx uint32) bool {
	text := findAnnotationTextFlat(file, idx, "Database")
	if text == "" {
		return false
	}
	compact := strings.Join(strings.Fields(text), "")
	return strings.Contains(compact, "exportSchema=false")
}

// RoomEntityChangedMigrationMissingRule detects @Entity columns whose names
// do not appear in any Room Migration(M, N) declaration in the project. A
// newly added or removed column without a corresponding migration update
// will fail Room's schema validation at runtime.
type RoomEntityChangedMigrationMissingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (low) base confidence. Detection cannot tell
// when a column was introduced; pre-existing columns left out of migration
// SQL also match. Inactive by default.
func (r *RoomEntityChangedMigrationMissingRule) Confidence() float64 { return api.ConfidenceMediumLow }

func (r *RoomEntityChangedMigrationMissingRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}

	migrationCorpus := collectRoomMigrationCorpus(index.Files)
	if migrationCorpus == "" {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Entity")) {
			continue
		}
		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
			if !hasAnnotationFlat(file, idx, "Entity") {
				return
			}
			ctor, _ := file.FlatFindChild(idx, "primary_constructor")
			if ctor == 0 {
				return
			}
			className := extractIdentifierFlat(file, idx)
			if className == "" {
				className = "Entity"
			}
			tableName := roomEntityTableName(file, idx, className)
			for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
				param := file.FlatNamedChild(ctor, i)
				if param == 0 || file.FlatType(param) != "class_parameter" {
					continue
				}
				if hasAnnotationFlat(file, param, "Ignore") {
					continue
				}
				column := roomEntityColumnName(file, param)
				if column == "" {
					continue
				}
				if migrationCorpusMentions(migrationCorpus, column) {
					continue
				}
				ctx.Emit(r.Finding(
					file,
					file.FlatRow(param)+1,
					file.FlatCol(param)+1,
					fmt.Sprintf("@Entity '%s' column '%s' is not referenced by any Migration(...) in the project; add a Migration(N-1, N) that updates table '%s' for this column.", className, column, tableName),
				))
			}
		})
	}
}

func collectRoomMigrationCorpus(files []*scanner.File) string {
	var b strings.Builder
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Migration")) {
			continue
		}
		appendMigrationDeclTexts(file, "object_declaration", &b)
		appendMigrationDeclTexts(file, "class_declaration", &b)
	}
	return b.String()
}

func appendMigrationDeclTexts(file *scanner.File, nodeType string, b *strings.Builder) {
	file.FlatWalkNodes(0, nodeType, func(idx uint32) {
		if !declarationExtendsRoomMigrationFlat(file, idx) {
			return
		}
		body, _ := file.FlatFindChild(idx, "class_body")
		if body == 0 {
			return
		}
		b.WriteString(file.FlatNodeText(body))
		b.WriteByte('\n')
	})
}

func declarationExtendsRoomMigrationFlat(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatForEachChild(idx, func(child uint32) {
		if found {
			return
		}
		if file.FlatType(child) != "delegation_specifier" {
			return
		}
		text := strings.Join(strings.Fields(file.FlatNodeText(child)), "")
		open := strings.Index(text, "Migration(")
		if open < 0 {
			return
		}
		if open > 0 {
			prev := text[open-1]
			if prev != '.' && prev != ':' && prev != ' ' {
				return
			}
		}
		args := text[open+len("Migration("):]
		closeParen := strings.Index(args, ")")
		if closeParen < 0 {
			return
		}
		parts := strings.Split(args[:closeParen], ",")
		if len(parts) != 2 {
			return
		}
		if !isIntLiteral(parts[0]) || !isIntLiteral(parts[1]) {
			return
		}
		found = true
	})
	return found
}

func isIntLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func roomEntityTableName(file *scanner.File, idx uint32, fallback string) string {
	if name := annotationStringArg(file, idx, "Entity", "tableName"); name != "" {
		return name
	}
	return fallback
}

func roomEntityColumnName(file *scanner.File, param uint32) string {
	if name := annotationStringArg(file, param, "ColumnInfo", "name"); name != "" {
		return name
	}
	return extractIdentifierFlat(file, param)
}

func annotationStringArg(file *scanner.File, idx uint32, annotation, arg string) string {
	text := findAnnotationTextFlat(file, idx, annotation)
	if text == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(text), "")
	key := arg + "=\""
	start := strings.Index(compact, key)
	if start < 0 {
		return ""
	}
	rest := compact[start+len(key):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func migrationCorpusMentions(corpus, column string) bool {
	if column == "" {
		return true
	}
	pos := 0
	for {
		rel := strings.Index(corpus[pos:], column)
		if rel < 0 {
			return false
		}
		idx := pos + rel
		end := idx + len(column)
		var before, after byte = ' ', ' '
		if idx > 0 {
			before = corpus[idx-1]
		}
		if end < len(corpus) {
			after = corpus[end]
		}
		if !isIdentChar(before) && !isIdentChar(after) {
			return true
		}
		pos = end
	}
}

// RoomDatabaseVersionNotBumpedRule detects @Database classes whose version
// literal is unchanged since HEAD while at least one @Entity-bearing source
// file has changed in the working tree. CI-only: skipped unless
// KRIT_CI_MODE=1 and git is available.
type RoomDatabaseVersionNotBumpedRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Schema-change
// detection is best-effort and depends on git diff state.
func (r *RoomDatabaseVersionNotBumpedRule) Confidence() float64 { return 0.65 }

// gitChangedSinceHEAD lists working-tree files changed relative to HEAD.
// Returned keys are absolute paths. Tests may override.
var gitChangedSinceHEAD = defaultGitChangedSinceHEAD

// gitFileAtHEAD returns the file's content at HEAD, or "" if absent.
// Tests may override.
var gitFileAtHEAD = defaultGitFileAtHEAD

func defaultGitChangedSinceHEAD() (map[string]bool, error) {
	rootBytes, err := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, err
	}
	root := strings.TrimSpace(string(rootBytes))
	if root == "" {
		return nil, fmt.Errorf("empty git toplevel")
	}
	out, err := exec.CommandContext(context.Background(), "git", "-C", root, "diff", "HEAD", "--name-only").Output()
	if err != nil {
		return nil, err
	}
	changed := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		changed[filepath.Join(root, line)] = true
	}
	return changed, nil
}

func defaultGitFileAtHEAD(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rootBytes, err := exec.CommandContext(context.Background(), "git", "-C", filepath.Dir(abs), "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(rootBytes))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	out, err := exec.CommandContext(context.Background(), "git", "-C", root, "show", "HEAD:"+filepath.ToSlash(rel)).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

var roomDatabaseVersionRe = regexp.MustCompile(`@Database\b[\s\S]*?\bversion\s*=\s*(\d+)`)

func extractRoomDatabaseVersionLiteral(text string) string {
	m := roomDatabaseVersionRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func absPathOrEmpty(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	return abs
}

func (r *RoomDatabaseVersionNotBumpedRule) check(ctx *api.Context) {
	if os.Getenv("KRIT_CI_MODE") != "1" {
		return
	}
	if len(ctx.ParsedFiles) == 0 {
		return
	}

	type dbInfo struct {
		file    *scanner.File
		idx     uint32
		version string
	}
	var dbs []dbInfo
	var entityFiles []*scanner.File

	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		hasDatabase := bytes.Contains(file.Content, []byte("Database"))
		hasEntity := bytes.Contains(file.Content, []byte("Entity"))
		if !hasDatabase && !hasEntity {
			continue
		}
		seenEntity := false
		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
			if hasDatabase {
				if text := findAnnotationTextFlat(file, idx, "Database"); text != "" {
					if v := extractRoomDatabaseVersionLiteral(text); v != "" {
						dbs = append(dbs, dbInfo{file, idx, v})
					}
				}
			}
			if hasEntity && !seenEntity && hasAnnotationFlat(file, idx, "Entity") {
				seenEntity = true
			}
		})
		if seenEntity {
			entityFiles = append(entityFiles, file)
		}
	}

	if len(dbs) == 0 || len(entityFiles) == 0 {
		return
	}

	changed, err := gitChangedSinceHEAD()
	if err != nil || len(changed) == 0 {
		return
	}

	entityChanged := false
	for _, f := range entityFiles {
		if changed[absPathOrEmpty(f.Path)] {
			entityChanged = true
			break
		}
	}
	if !entityChanged {
		return
	}

	for _, db := range dbs {
		dbAbs := absPathOrEmpty(db.file.Path)
		if changed[dbAbs] {
			prior, err := gitFileAtHEAD(db.file.Path)
			if err != nil {
				continue
			}
			priorVersion := extractRoomDatabaseVersionLiteral(prior)
			if priorVersion == "" || priorVersion != db.version {
				continue
			}
		}
		ctx.Emit(r.Finding(
			db.file,
			db.file.FlatRow(db.idx)+1,
			db.file.FlatCol(db.idx)+1,
			fmt.Sprintf("@Database version=%s is unchanged but @Entity sources have changed since HEAD; bump the version and add a Migration.", db.version),
		))
	}
}

// RoomMigrationUsesExecSQLWithInterpolationRule detects `db.execSQL(...)`
// calls inside a class or object that extends Room's `Migration`, where the
// SQL string uses Kotlin string interpolation. Interpolated DDL inside a
// migration is brittle and a SQL-injection foot-gun.
type RoomMigrationUsesExecSQLWithInterpolationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection matches
// on the call name and an enclosing supertype named `Migration`, without
// confirming androidx.room.migration.Migration resolves the type.
func (r *RoomMigrationUsesExecSQLWithInterpolationRule) Confidence() float64 {
	return api.ConfidenceHigh
}

func enclosingMigrationOwnerFlat(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration":
			for child := file.FlatFirstChild(cur); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "delegation_specifier" {
					continue
				}
				userType, _ := file.FlatFindChild(child, "user_type")
				if userType == 0 {
					if ctor, okc := file.FlatFindChild(child, "constructor_invocation"); okc {
						userType, _ = file.FlatFindChild(ctor, "user_type")
					}
				}
				if userType == 0 {
					continue
				}
				if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
					if file.FlatNodeTextEquals(ident, "Migration") {
						return true
					}
				}
			}
		}
	}
	return false
}

func (r *RoomFallbackToDestructiveMigrationRule) Confidence() float64 { return api.ConfidenceHigh }

func enclosedInBuildConfigDebugGuard(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "if_expression" {
			continue
		}
		cond := privacyIfConditionText(file, cur)
		if strings.Contains(cond, "BuildConfig.DEBUG") {
			return true
		}
	}
	return false
}
