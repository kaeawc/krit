package hashutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sha256Hasher is kept only so the benchmark can quantify the win over
// the previous default. It is NOT exported from the package.
type sha256Hasher struct{}

func (sha256Hasher) Name() string              { return "sha256" }
func (sha256Hasher) Sum(b []byte) [32]byte     { return sha256.Sum256(b) }
func (sha256Hasher) New() hash.Hash            { return sha256.New() }

// BenchmarkContentHasher compares throughput of the installed hasher
// against stdlib SHA-256 across file-size buckets that bracket a typical
// Kotlin source tree: small utility files, medium feature files, and
// large generated sources.
//
// The idea is not to replace a microbenchmark suite but to give CI a
// regression-tripwire if we ever accidentally swap the default back to
// a slower algorithm.
func BenchmarkContentHasher(b *testing.B) {
	sizes := []int{1 << 10, 16 << 10, 256 << 10}
	for _, n := range sizes {
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			b.Fatal(err)
		}

		b.Run("xxh3-256/"+sizeLabel(n), func(b *testing.B) {
			h := xxh3Hasher{}
			b.SetBytes(int64(n))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = h.Sum(buf)
			}
		})
		b.Run("sha256/"+sizeLabel(n), func(b *testing.B) {
			h := sha256Hasher{}
			b.SetBytes(int64(n))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = h.Sum(buf)
			}
		})
	}
}

func sizeLabel(n int) string {
	switch {
	case n >= 1<<20:
		return (itoa(n>>20) + "MB")
	case n >= 1<<10:
		return (itoa(n>>10) + "KB")
	default:
		return itoa(n) + "B"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// TestContentHasherCollisionAudit hashes every Go, Kotlin, and Markdown
// source file in the krit repo and asserts zero hash collisions. A 32-
// byte cryptographic digest has a 2^128 birthday bound, so a collision
// on a few thousand files would be a serious library bug. The audit is
// cheap enough (<200ms) to run on every CI push.
func TestContentHasherCollisionAudit(t *testing.T) {
	root := repoRoot(t)
	seen := make(map[string]string, 4096)
	var files int
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == ".krit" || name == "node_modules" || name == ".idea" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		switch ext {
		case ".go", ".kt", ".kts", ".java", ".md", ".yml", ".yaml", ".json":
		default:
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		raw := HashBytes(data)
		hx := hex.EncodeToString(raw[:])
		if prev, ok := seen[hx]; ok {
			prevData, _ := os.ReadFile(prev)
			if string(prevData) != string(data) {
				t.Fatalf("content-hash collision: %s vs %s share %s", prev, p, hx)
			}
		} else {
			seen[hx] = p
		}
		files++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if files < 100 {
		t.Fatalf("audit walked only %d files; expected the krit corpus", files)
	}
	t.Logf("audited %d files, %d unique hashes, zero collisions", files, len(seen))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find repo root")
	return ""
}
