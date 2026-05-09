package typeinfer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	typeIndexCacheDirName = "type-index-cache"
	// Bumped to 2 when MemberInfo gained Params []ParamInfo and
	// TypeParameters []string. Older cache payloads decode the prior
	// MemberInfo shape and are silently dropped on version mismatch.
	typeIndexCacheVersion = 2
)

var (
	typeIndexHits      atomic.Int64
	typeIndexMisses    atomic.Int64
	typeIndexEntries   atomic.Int64
	typeIndexBytes     atomic.Int64
	typeIndexLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(typeIndexCacheRegistered{})
}

type typeIndexCacheRegistered struct{}

func (typeIndexCacheRegistered) Name() string { return typeIndexCacheDirName }
func (typeIndexCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearTypeIndexCache(TypeIndexCacheDir(ctx.RepoDir))
}
func (typeIndexCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Entries:       int(typeIndexEntries.Load()),
		Bytes:         typeIndexBytes.Load(),
		Hits:          typeIndexHits.Load(),
		Misses:        typeIndexMisses.Load(),
		LastWriteUnix: typeIndexLastWrite.Load(),
	}
}

// TypeIndexCacheDir returns the per-file source type-index cache directory.
func TypeIndexCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", typeIndexCacheDirName)
}

func ClearTypeIndexCache(dir string) error {
	if dir == "" {
		return nil
	}
	typeIndexEntries.Store(0)
	typeIndexBytes.Store(0)
	return os.RemoveAll(dir)
}

type packedFileTypeInfo struct {
	Version     int
	Path        string
	ImportTable *ImportTable
	RootScope   packedScope
	Classes     []*ClassInfo
	SealedSubs  map[string][]string
	EnumEntries map[string][]string
	TypeAliases map[string]*ResolvedType
	Functions   map[string]*ResolvedType
	Extensions  []*ExtensionFuncInfo
}

type packedScope struct {
	Entries        map[string]*ResolvedType
	SmartCasts     map[string]bool
	SmartCastTypes map[string]*ResolvedType
	StartByte      uint32
	EndByte        uint32
	Children       []packedScope
}

func packFileTypeInfo(info *FileTypeInfo) packedFileTypeInfo {
	if info == nil {
		return packedFileTypeInfo{Version: typeIndexCacheVersion}
	}
	return packedFileTypeInfo{
		Version:     typeIndexCacheVersion,
		Path:        info.Path,
		ImportTable: info.ImportTable,
		RootScope:   packScope(info.RootScope),
		Classes:     info.Classes,
		SealedSubs:  info.SealedSubs,
		EnumEntries: info.EnumEntries,
		TypeAliases: info.TypeAliases,
		Functions:   info.Functions,
		Extensions:  info.Extensions,
	}
}

func packScope(scope *ScopeTable) packedScope {
	if scope == nil {
		return packedScope{}
	}
	out := packedScope{
		Entries:        scope.Entries,
		SmartCasts:     scope.SmartCasts,
		SmartCastTypes: scope.SmartCastTypes,
		StartByte:      scope.StartByte,
		EndByte:        scope.EndByte,
		Children:       make([]packedScope, 0, len(scope.Children)),
	}
	for _, child := range scope.Children {
		out.Children = append(out.Children, packScope(child))
	}
	return out
}

func unpackFileTypeInfo(p packedFileTypeInfo) (*FileTypeInfo, bool) {
	if p.Version != typeIndexCacheVersion || p.Path == "" {
		return nil, false
	}
	return &FileTypeInfo{
		Path:        p.Path,
		ImportTable: p.ImportTable,
		RootScope:   unpackScope(p.RootScope, nil),
		Classes:     p.Classes,
		SealedSubs:  p.SealedSubs,
		EnumEntries: p.EnumEntries,
		TypeAliases: p.TypeAliases,
		Functions:   p.Functions,
		Extensions:  p.Extensions,
	}, true
}

func unpackScope(p packedScope, parent *ScopeTable) *ScopeTable {
	scope := &ScopeTable{
		Parent:         parent,
		Entries:        p.Entries,
		SmartCasts:     p.SmartCasts,
		SmartCastTypes: p.SmartCastTypes,
		StartByte:      p.StartByte,
		EndByte:        p.EndByte,
		Children:       make([]*ScopeTable, 0, len(p.Children)),
	}
	if scope.Entries == nil {
		scope.Entries = make(map[string]*ResolvedType)
	}
	if scope.SmartCasts == nil {
		scope.SmartCasts = make(map[string]bool)
	}
	if scope.SmartCastTypes == nil {
		scope.SmartCastTypes = make(map[string]*ResolvedType)
	}
	for _, child := range p.Children {
		scope.Children = append(scope.Children, unpackScope(child, scope))
	}
	return scope
}

func typeIndexCacheKey(file *scanner.File) string {
	contentHash := hashutil.Default().HashContent(file.Path, file.Content)
	return hashutil.HashHex([]byte(file.Path + "\x00" + contentHash))
}

func typeIndexCachePath(cacheDir, key string) string {
	return filepath.Join(cacheDir, key[:2], key+".gob.zst")
}

func loadFileTypeInfoCached(cacheDir string, file *scanner.File) (*FileTypeInfo, bool) {
	if cacheDir == "" || file == nil {
		typeIndexMisses.Add(1)
		return nil, false
	}
	key := typeIndexCacheKey(file)
	path := typeIndexCachePath(cacheDir, key)
	f, err := os.Open(path)
	if err != nil {
		typeIndexMisses.Add(1)
		return nil, false
	}
	defer f.Close()
	var packed packedFileTypeInfo
	if err := cacheutil.DecodeZstdGob(f, &packed); err != nil {
		typeIndexMisses.Add(1)
		return nil, false
	}
	info, ok := unpackFileTypeInfo(packed)
	if !ok || info.Path != file.Path {
		typeIndexMisses.Add(1)
		return nil, false
	}
	if st, err := f.Stat(); err == nil {
		typeIndexBytes.Add(st.Size())
	}
	typeIndexEntries.Add(1)
	typeIndexHits.Add(1)
	return info, true
}

func saveFileTypeInfoCached(cacheDir string, file *scanner.File, info *FileTypeInfo) error {
	if cacheDir == "" || file == nil || info == nil {
		return nil
	}
	key := typeIndexCacheKey(file)
	path := typeIndexCachePath(cacheDir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir type-index cache: %w", err)
	}
	blob, err := cacheutil.EncodeZstdGob(packFileTypeInfo(info))
	if err != nil {
		return fmt.Errorf("encode type-index cache: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, blob, 0o644); err != nil {
		return fmt.Errorf("write type-index cache: %w", err)
	}
	typeIndexLastWrite.Store(time.Now().Unix())
	typeIndexEntries.Add(1)
	typeIndexBytes.Add(int64(len(blob)))
	return nil
}
