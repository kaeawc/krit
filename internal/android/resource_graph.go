package android

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

const (
	resourceGraphCacheDirName = "resource-graph-cache"
	resourceGraphVersion      = 1
)

var (
	resourceGraphHits      atomic.Int64
	resourceGraphMisses    atomic.Int64
	resourceGraphBytes     atomic.Int64
	resourceGraphLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(resourceGraphCacheRegistered{})
}

type resourceGraphCacheRegistered struct{}

func (resourceGraphCacheRegistered) Name() string { return resourceGraphCacheDirName }
func (resourceGraphCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearResourceGraphCache(ResourceGraphCacheDir(ctx.RepoDir))
}
func (resourceGraphCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Bytes:         resourceGraphBytes.Load(),
		Hits:          resourceGraphHits.Load(),
		Misses:        resourceGraphMisses.Load(),
		LastWriteUnix: resourceGraphLastWrite.Load(),
	}
}

type ResourceID struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type ResourceFileNode struct {
	Path    string       `json:"path"`
	Defines []ResourceID `json:"defines"`
	Refs    []ResourceID `json:"refs"`
}

type ResourceGraph struct {
	Files       map[string]ResourceFileNode `json:"files"`
	DefinedIn   map[ResourceID][]string     `json:"definedIn"`
	ReverseDeps map[string][]string         `json:"reverseDeps"`
}

type ResourceGraphBuilder interface {
	Build(root string, resDirs []string, hashes map[string]string) (*ResourceGraph, error)
}

type ResourceDependencyResolver interface {
	AffectedFiles(changed []string) []string
}

type ResourceGraphCache interface {
	Load(root, key string) (*ResourceGraph, bool)
	Save(root, key string, graph *ResourceGraph) error
}

type XMLResourceGraphBuilder struct{}

func (XMLResourceGraphBuilder) Build(_ string, resDirs []string, _ map[string]string) (*ResourceGraph, error) {
	g := &ResourceGraph{
		Files:       make(map[string]ResourceFileNode),
		DefinedIn:   make(map[ResourceID][]string),
		ReverseDeps: make(map[string][]string),
	}
	for _, resDir := range resDirs {
		if err := filepath.WalkDir(resDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".xml" {
				return nil
			}
			node, err := parseResourceGraphFile(resDir, path)
			if err != nil {
				return err
			}
			g.Files[path] = node
			for _, id := range node.Defines {
				g.DefinedIn[id] = appendUniqueString(g.DefinedIn[id], path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	for path, node := range g.Files {
		for _, ref := range node.Refs {
			for _, definedPath := range g.DefinedIn[ref] {
				if definedPath == path {
					continue
				}
				g.ReverseDeps[definedPath] = appendUniqueString(g.ReverseDeps[definedPath], path)
			}
		}
	}
	sortResourceGraph(g)
	return g, nil
}

type CachedResourceGraphBuilder struct {
	Builder ResourceGraphBuilder
	Cache   ResourceGraphCache
}

func (b CachedResourceGraphBuilder) Build(root string, resDirs []string, hashes map[string]string) (*ResourceGraph, error) {
	builder := b.Builder
	if builder == nil {
		builder = XMLResourceGraphBuilder{}
	}
	cache := b.Cache
	if cache == nil {
		cache = DiskResourceGraphCache{}
	}
	key, ok := ResourceGraphKey(resDirs, hashes)
	if ok {
		if graph, hit := cache.Load(root, key); hit {
			resourceGraphHits.Add(1)
			return graph, nil
		}
		resourceGraphMisses.Add(1)
	}
	graph, err := builder.Build(root, resDirs, hashes)
	if err != nil {
		return nil, err
	}
	if ok {
		_ = cache.Save(root, key, graph)
	}
	return graph, nil
}

type GraphResolver struct {
	Graph *ResourceGraph
}

func (r GraphResolver) AffectedFiles(changed []string) []string {
	if r.Graph == nil {
		return append([]string(nil), changed...)
	}
	seen := make(map[string]bool, len(changed))
	queue := append([]string(nil), changed...)
	for _, path := range changed {
		seen[path] = true
	}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		for _, dep := range r.Graph.ReverseDeps[path] {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			queue = append(queue, dep)
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

type DiskResourceGraphCache struct{}

type resourceGraphPayload struct {
	Version int            `json:"version"`
	Key     string         `json:"key"`
	Graph   *ResourceGraph `json:"graph"`
}

func ResourceGraphCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", resourceGraphCacheDirName)
}

func ClearResourceGraphCache(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	resourceGraphBytes.Store(0)
	return nil
}

func (DiskResourceGraphCache) Load(root, key string) (*ResourceGraph, bool) {
	if root == "" || key == "" {
		return nil, false
	}
	f, err := os.Open(resourceGraphEntryPath(root, key))
	if err != nil {
		return nil, false
	}
	defer f.Close()
	var payload resourceGraphPayload
	if err := cacheutil.DecodeZstdGob(f, &payload); err != nil {
		return nil, false
	}
	if payload.Version != resourceGraphVersion || payload.Key != key || payload.Graph == nil {
		return nil, false
	}
	return payload.Graph, true
}

func (DiskResourceGraphCache) Save(root, key string, graph *ResourceGraph) error {
	if root == "" || key == "" || graph == nil {
		return nil
	}
	path := resourceGraphEntryPath(root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := cacheutil.EncodeZstdGob(resourceGraphPayload{Version: resourceGraphVersion, Key: key, Graph: graph})
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, raw, 0o644); err != nil {
		return err
	}
	resourceGraphBytes.Add(int64(len(raw)))
	resourceGraphLastWrite.Store(time.Now().Unix())
	return nil
}

func ResourceGraphKey(resDirs []string, hashes map[string]string) (string, bool) {
	h := hashutil.Hasher().New()
	sorted := append([]string(nil), resDirs...)
	sort.Strings(sorted)
	memo := hashutil.Default()
	for _, dir := range sorted {
		h.Write([]byte(dir))
		h.Write([]byte{0})
		var files []string
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".xml" {
				return nil
			}
			files = append(files, path)
			return nil
		}); err != nil {
			return "", false
		}
		sort.Strings(files)
		for _, path := range files {
			fileHash := hashes[path]
			if fileHash == "" {
				hx, err := memo.HashFile(path, nil)
				if err != nil {
					return "", false
				}
				fileHash = hx
			}
			h.Write([]byte(path))
			h.Write([]byte{0})
			h.Write([]byte(fileHash))
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func resourceGraphEntryPath(root, key string) string {
	if len(key) >= 2 {
		return filepath.Join(ResourceGraphCacheDir(root), "entries", key[:2], key[2:]+".bin")
	}
	return filepath.Join(ResourceGraphCacheDir(root), "entries", key+".bin")
}

var resourceRefRE = regexp.MustCompile(`[@?]([A-Za-z0-9_]+)/([A-Za-z0-9_.]+)`)

func parseResourceGraphFile(resDir, path string) (ResourceFileNode, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ResourceFileNode{}, err
	}
	node := ResourceFileNode{Path: path}
	node.Defines = append(node.Defines, resourceDefinitions(resDir, path, raw)...)
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ResourceFileNode{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			for _, attr := range t.Attr {
				node.Refs = append(node.Refs, resourceRefs(attr.Value)...)
				if t.Name.Local == "style" && attr.Name.Local == "parent" {
					node.Refs = append(node.Refs, styleParentRef(attr.Value)...)
				}
			}
		case xml.CharData:
			node.Refs = append(node.Refs, resourceRefs(string(t))...)
		}
	}
	node.Defines = uniqueResourceIDs(node.Defines)
	node.Refs = uniqueResourceIDs(node.Refs)
	return node, nil
}

func resourceDefinitions(resDir, path string, raw []byte) []ResourceID {
	rel, err := filepath.Rel(resDir, path)
	if err != nil {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return nil
	}
	dirType := strings.Split(parts[0], "-")[0]
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if dirType != "values" {
		return []ResourceID{{Type: dirType, Name: name}}
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var out []ResourceID
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return out
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		var resName string
		var itemType string
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "name":
				resName = attr.Value
			case "type":
				itemType = attr.Value
			}
		}
		if resName == "" {
			continue
		}
		resType := start.Name.Local
		if resType == "item" && itemType != "" {
			resType = itemType
		}
		out = append(out, ResourceID{Type: resType, Name: resName})
	}
	return out
}

func resourceRefs(text string) []ResourceID {
	var out []ResourceID
	for _, match := range resourceRefRE.FindAllStringSubmatch(text, -1) {
		if len(match) != 3 {
			continue
		}
		out = append(out, ResourceID{Type: match[1], Name: match[2]})
	}
	return out
}

func styleParentRef(value string) []ResourceID {
	if value == "" || strings.HasPrefix(value, "@") || strings.HasPrefix(value, "?") {
		return nil
	}
	return []ResourceID{{Type: "style", Name: strings.TrimPrefix(value, "style/")}}
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func uniqueResourceIDs(values []ResourceID) []ResourceID {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[ResourceID]bool, len(values))
	out := make([]ResourceID, 0, len(values))
	for _, value := range values {
		if value.Type == "" || value.Name == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortResourceGraph(g *ResourceGraph) {
	for id := range g.DefinedIn {
		sort.Strings(g.DefinedIn[id])
	}
	for path := range g.ReverseDeps {
		sort.Strings(g.ReverseDeps[path])
	}
}
