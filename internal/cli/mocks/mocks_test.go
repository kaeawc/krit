package mocks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/test/kotlin/com/example/UserServiceTest.kt", `
package com.example

import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import org.mockito.Mock

class UserServiceTest {
    private val api = mockk<Api>()
    private val unusedCache: DiskCache = mockk()

    @Mock
    lateinit var clock: Clock

    fun loads() {
        every { api.fetch() } returns "ok"
        verify { api.fetch() }
        clock.instant()
    }
}
`)
	writeFile(t, root, "src/test/java/com/example/RepositoryTest.java", `
package com.example;

import org.mockito.Mockito;

class RepositoryTest {
    Api api = Mockito.mock(Api.class);
    DiskCache unused = Mockito.mock(DiskCache.class);

    void loads() {
        api.fetch();
    }
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalMocks != 5 {
		t.Fatalf("expected 5 mocks, got %d: %+v", report.TotalMocks, report.Mocks)
	}
	if report.ByLibrary["mockk"] != 2 || report.ByLibrary["mockito"] != 3 {
		t.Fatalf("unexpected library counts: %+v", report.ByLibrary)
	}
	if len(report.Unused) != 2 {
		t.Fatalf("expected 2 unused mocks, got %d: %+v", len(report.Unused), report.Unused)
	}
	if !hasUnused(report, "unusedCache", "DiskCache") || !hasUnused(report, "unused", "DiskCache") {
		t.Fatalf("missing expected unused DiskCache mocks: %+v", report.Unused)
	}
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

func hasUnused(report Report, name, target string) bool {
	for _, mock := range report.Unused {
		if mock.Name == name && mock.TargetType == target {
			return true
		}
	}
	return false
}
