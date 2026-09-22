package cache

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/store"
)

func TestFindingsStoreFileHash_FoldsOracleFactsOnlyWhenPresent(t *testing.T) {
	dir := t.TempDir()
	dependency := filepath.Join(dir, "Dependency.kt")
	caller := filepath.Join(dir, "Caller.kt")
	if err := os.WriteFile(dependency, []byte("package demo\nfun dependency() = \"value\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	callerSource := []byte("package demo\nfun caller() = dependency().length\n")
	if err := os.WriteFile(caller, callerSource, 0o644); err != nil {
		t.Fatal(err)
	}
	contentHash, err := computeFileHash32(caller)
	if err != nil {
		t.Fatalf("computeFileHash32: %v", err)
	}

	beforeFacts := oracle.BlobHash(&oracle.File{Expressions: map[string]*oracle.ExpressionType{
		"2:16": {Type: "kotlin.String", Nullable: false, StartByte: 28, EndByte: 40},
	}})
	before := store.Key{FileHash: foldOracleBlobHash(contentHash, beforeFacts), Kind: store.KindIncremental}
	if err := os.WriteFile(dependency, []byte("package demo\nfun dependency() = null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	callerAfter, err := os.ReadFile(caller)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(callerSource, callerAfter) {
		t.Fatal("caller bytes changed while dependency facts changed")
	}
	contentHashAfter, err := computeFileHash32(caller)
	if err != nil {
		t.Fatalf("computeFileHash32 after dependency edit: %v", err)
	}
	if contentHashAfter != contentHash {
		t.Fatal("caller content hash changed while dependency facts changed")
	}
	afterFacts := oracle.BlobHash(&oracle.File{Expressions: map[string]*oracle.ExpressionType{
		"2:16": {Type: "kotlin.String", Nullable: true, StartByte: 28, EndByte: 40},
	}})
	after := store.Key{FileHash: foldOracleBlobHash(contentHashAfter, afterFacts), Kind: store.KindIncremental}
	if before == after {
		t.Fatalf("oracle fact change left findings store.Key unchanged: %x", before.FileHash)
	}
	if got := foldOracleBlobHash(contentHash, ""); got != contentHash {
		t.Fatalf("no-oracle key changed: got %x want raw content hash %x", got, contentHash)
	}
}
