package gen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunModuleReadmeRendersSummary(t *testing.T) {
	root := writeModuleReadmeProject(t)
	var stdout, stderr bytes.Buffer
	code := runModuleReadme([]string{":core", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runModuleReadme code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"# :core",
		"**Depends on:** :common",
		"**Depended on by:** :app",
		"- class `UserRepository`",
		"- fun `loadUser`",
		"- `core/src/test/kotlin/com/example/core/UserRepositoryTest.kt` (2 tests)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "privateHelper") {
		t.Fatalf("private API leaked into README:\n%s", out)
	}
	if strings.Contains(out, "buildHelper") {
		t.Fatalf("build script symbol leaked into README:\n%s", out)
	}
}

func TestRunModuleReadmeUnknownModule(t *testing.T) {
	root := writeModuleReadmeProject(t)
	var stdout, stderr bytes.Buffer
	code := runModuleReadme([]string{":missing", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero for unknown module, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown module ":missing"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunModuleReadmeNoSettings(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runModuleReadme([]string{":core", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero when settings file is absent, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no settings.gradle(.kts) found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunWalkthroughPlain(t *testing.T) {
	root := writeWalkthroughProject(t)
	var stdout, stderr bytes.Buffer
	code := runWalkthrough([]string{"--n", "3", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWalkthrough code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Seed: com.example.UserRepository",
		"Why this file: highest class-like fan-in",
		"Reading order:",
		"1. src/main/kotlin/com/example/UserRepository.kt",
		"2. src/main/kotlin/com/example/SqlUserStore.kt",
		"3. src/main/kotlin/com/example/UserMapper.kt",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("walkthrough missing %q:\n%s", want, out)
		}
	}
}

func TestRunWalkthroughJSON(t *testing.T) {
	root := writeWalkthroughProject(t)
	var stdout, stderr bytes.Buffer
	code := runWalkthrough([]string{"--n=2", "--report=json", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWalkthrough code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"symbol": "com.example.UserRepository"`) {
		t.Fatalf("json walkthrough missing seed symbol:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"files":`) {
		t.Fatalf("json walkthrough missing files:\n%s", stdout.String())
	}
}

func writeModuleReadmeProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "settings.gradle.kts", `pluginManagement { repositories { google() } }
dependencyResolutionManagement { repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS) }
rootProject.name = "fixture"
include(":common", ":core", ":app")
`)
	writeFile(t, root, "core/build.gradle.kts", `plugins { kotlin("jvm") }
dependencies {
    implementation(project(":common"))
}

fun buildHelper() = "not api"
`)
	writeFile(t, root, "app/build.gradle.kts", `plugins { kotlin("jvm") }
dependencies {
    implementation(project(":core"))
}
`)
	writeFile(t, root, "common/build.gradle.kts", `plugins { kotlin("jvm") }
`)
	writeFile(t, root, "core/src/main/kotlin/com/example/core/UserRepository.kt", `package com.example.core

class UserRepository

fun loadUser(): String = "user"

private fun privateHelper(): String = "hidden"
`)
	writeFile(t, root, "core/src/test/kotlin/com/example/core/UserRepositoryTest.kt", `package com.example.core

class UserRepositoryTest {
    fun loadsUser() {}
    fun savesUser() {}
}
`)
	return root
}

func writeWalkthroughProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "src/main/kotlin/com/example/UserRepository.kt", `package com.example

class UserRepository(
    private val store: SqlUserStore,
    private val mapper: UserMapper,
) {
    fun load(): UserDto {
        val row = store.load()
        val fallback = store.load()
        return mapper.map(row, fallback)
    }
}

class UserDto
`)
	writeFile(t, root, "src/main/kotlin/com/example/SqlUserStore.kt", `package com.example

class SqlUserStore {
    fun load(): String = "row"
}
`)
	writeFile(t, root, "src/main/kotlin/com/example/UserMapper.kt", `package com.example

class UserMapper {
    fun map(row: String, fallback: String): UserDto = UserDto()
}
`)
	writeFile(t, root, "src/main/kotlin/com/example/Controller.kt", `package com.example

class Controller(private val repository: UserRepository)
`)
	writeFile(t, root, "src/main/kotlin/com/example/Worker.kt", `package com.example

class Worker {
    fun run(repository: UserRepository) = repository.load()
}
`)
	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
