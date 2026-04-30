package rules

import (
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func writeAndParseKotlin(t *testing.T, dir, name, content string) *scanner.File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return file
}

func runRoomDatabaseVersionRule(files []*scanner.File) []scanner.Finding {
	for _, r := range v2.Registry {
		if r.ID != "RoomDatabaseVersionNotBumped" {
			continue
		}
		ctx := &v2.Context{
			ParsedFiles: files,
			Collector:   scanner.NewFindingCollector(0),
			Rule:        r,
		}
		r.Check(ctx)
		return ctx.Collector.Columns().Findings()
	}
	return nil
}

func withCIMode(t *testing.T) {
	t.Helper()
	prev, had := os.LookupEnv("KRIT_CI_MODE")
	os.Setenv("KRIT_CI_MODE", "1")
	t.Cleanup(func() {
		if had {
			os.Setenv("KRIT_CI_MODE", prev)
		} else {
			os.Unsetenv("KRIT_CI_MODE")
		}
	})
}

func stubGit(t *testing.T, changed map[string]bool, atHEAD map[string]string) {
	t.Helper()
	prevChanged := gitChangedSinceHEAD
	prevHEAD := gitFileAtHEAD
	abs := make(map[string]bool, len(changed))
	for p := range changed {
		a, _ := filepath.Abs(p)
		abs[a] = true
	}
	gitChangedSinceHEAD = func() (map[string]bool, error) { return abs, nil }
	gitFileAtHEAD = func(path string) (string, error) {
		a, _ := filepath.Abs(path)
		if v, ok := atHEAD[a]; ok {
			return v, nil
		}
		return "", nil
	}
	t.Cleanup(func() {
		gitChangedSinceHEAD = prevChanged
		gitFileAtHEAD = prevHEAD
	})
}

const roomDbV2 = `package db

annotation class Database(val entities: Array<kotlin.reflect.KClass<*>>, val version: Int)

@Database(entities = [User::class], version = 2)
abstract class AppDb
`

const roomEntityChanged = `package db

annotation class Entity

@Entity
data class User(val id: Long, val email: String)
`

func TestRoomDatabaseVersionNotBumped_Skipped_WhenCIModeOff(t *testing.T) {
	dir := t.TempDir()
	dbFile := writeAndParseKotlin(t, dir, "AppDb.kt", roomDbV2)
	entityFile := writeAndParseKotlin(t, dir, "User.kt", roomEntityChanged)
	stubGit(t,
		map[string]bool{entityFile.Path: true},
		map[string]string{},
	)
	if findings := runRoomDatabaseVersionRule([]*scanner.File{dbFile, entityFile}); len(findings) != 0 {
		t.Fatalf("expected 0 findings without CI mode, got %d", len(findings))
	}
}

func TestRoomDatabaseVersionNotBumped_FlagsUnchangedDatabase(t *testing.T) {
	withCIMode(t)
	dir := t.TempDir()
	dbFile := writeAndParseKotlin(t, dir, "AppDb.kt", roomDbV2)
	entityFile := writeAndParseKotlin(t, dir, "User.kt", roomEntityChanged)
	stubGit(t,
		map[string]bool{entityFile.Path: true},
		map[string]string{},
	)
	findings := runRoomDatabaseVersionRule([]*scanner.File{dbFile, entityFile})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].File != dbFile.Path {
		t.Fatalf("expected finding on AppDb.kt, got %s", findings[0].File)
	}
}

func TestRoomDatabaseVersionNotBumped_VersionAlsoBumped(t *testing.T) {
	withCIMode(t)
	dir := t.TempDir()
	dbFile := writeAndParseKotlin(t, dir, "AppDb.kt", roomDbV2)
	entityFile := writeAndParseKotlin(t, dir, "User.kt", roomEntityChanged)
	priorDb := `package db
annotation class Database(val entities: Array<kotlin.reflect.KClass<*>>, val version: Int)
@Database(entities = [User::class], version = 1)
abstract class AppDb
`
	stubGit(t,
		map[string]bool{entityFile.Path: true, dbFile.Path: true},
		map[string]string{mustAbs(t, dbFile.Path): priorDb},
	)
	if findings := runRoomDatabaseVersionRule([]*scanner.File{dbFile, entityFile}); len(findings) != 0 {
		t.Fatalf("expected 0 findings when version is bumped, got %d", len(findings))
	}
}

func TestRoomDatabaseVersionNotBumped_NoEntityChange(t *testing.T) {
	withCIMode(t)
	dir := t.TempDir()
	dbFile := writeAndParseKotlin(t, dir, "AppDb.kt", roomDbV2)
	entityFile := writeAndParseKotlin(t, dir, "User.kt", roomEntityChanged)
	stubGit(t,
		map[string]bool{},
		map[string]string{},
	)
	if findings := runRoomDatabaseVersionRule([]*scanner.File{dbFile, entityFile}); len(findings) != 0 {
		t.Fatalf("expected 0 findings when no entity changed, got %d", len(findings))
	}
}

func TestRoomDatabaseVersionNotBumped_VersionUnchangedInDiff(t *testing.T) {
	withCIMode(t)
	dir := t.TempDir()
	dbFile := writeAndParseKotlin(t, dir, "AppDb.kt", roomDbV2)
	entityFile := writeAndParseKotlin(t, dir, "User.kt", roomEntityChanged)
	priorDb := `package db
annotation class Database(val entities: Array<kotlin.reflect.KClass<*>>, val version: Int)
@Database(entities = [User::class], version = 2)
abstract class AppDb
`
	stubGit(t,
		map[string]bool{entityFile.Path: true, dbFile.Path: true},
		map[string]string{mustAbs(t, dbFile.Path): priorDb},
	)
	findings := runRoomDatabaseVersionRule([]*scanner.File{dbFile, entityFile})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when @Database file changed without bumping version, got %d", len(findings))
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("filepath.Abs(%s): %v", p, err)
	}
	return abs
}
