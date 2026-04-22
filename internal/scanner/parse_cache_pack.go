package scanner

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
)

const (
	parsePackMagic       uint32 = 0x4b50504b // "KPPK"
	parsePackVersion     uint16 = 1
	parsePackSubdir             = "packs-v1"
	parsePackBucketCount        = 256
	parsePackHeaderFixed        = 4 + 2 + 4
	parsePackEntryFixed         = 2 + 8 + 8 + 4
	parsePackExt                = ".pack"
)

var (
	errParsePackCorrupt = errors.New("parse cache pack corrupt")
	parsePackPathLocks  sync.Map
)

type parsePackEntry struct {
	offset uint64
	length uint64
	crc    uint32
}

type parseEncodedEntryWrite struct {
	hash string
	data []byte
}

type parsePackItem struct {
	key  string
	data []byte
	crc  uint32
}

type parsePackRecord struct {
	hash string
	size int64
}

type parsePackHandle struct {
	path    string
	mu      sync.RWMutex
	loaded  bool
	missing bool
	index   map[string]parsePackEntry
	data    []byte
}

type parsePackStore struct {
	dir   string
	packs [parsePackBucketCount]*parsePackHandle
}

func newParsePackStore(langDir string) *parsePackStore {
	if langDir == "" {
		return nil
	}
	ps := &parsePackStore{dir: filepath.Join(langDir, parsePackSubdir)}
	for i := 0; i < parsePackBucketCount; i++ {
		ps.packs[i] = &parsePackHandle{
			path: filepath.Join(ps.dir, fmt.Sprintf("%02x%s", i, parsePackExt)),
		}
	}
	return ps
}

func (ps *parsePackStore) packForHash(hash string) (*parsePackHandle, error) {
	if ps == nil {
		return nil, nil
	}
	if len(hash) < 2 {
		return nil, fmt.Errorf("parse cache hash too short: %q", hash)
	}
	n, err := strconv.ParseUint(hash[:2], 16, 8)
	if err != nil {
		return nil, fmt.Errorf("parse cache hash not hex: %q", hash)
	}
	return ps.packs[n], nil
}

func (h *parsePackHandle) ensureLoaded() error {
	h.mu.RLock()
	if h.loaded {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.loaded {
		return nil
	}
	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			h.loaded = true
			h.missing = true
			h.index = nil
			h.data = nil
			return nil
		}
		return err
	}
	idx, err := parseParsePackHeader(data)
	if err != nil {
		return err
	}
	h.data = data
	h.index = idx
	h.loaded = true
	h.missing = false
	return nil
}

func parseParsePackHeader(data []byte) (map[string]parsePackEntry, error) {
	if len(data) < parsePackHeaderFixed {
		return nil, errParsePackCorrupt
	}
	if binary.LittleEndian.Uint32(data[0:4]) != parsePackMagic {
		return nil, errParsePackCorrupt
	}
	if binary.LittleEndian.Uint16(data[4:6]) != parsePackVersion {
		return nil, errParsePackCorrupt
	}
	n := binary.LittleEndian.Uint32(data[6:10])
	idx := make(map[string]parsePackEntry, n)
	pos := parsePackHeaderFixed
	for i := uint32(0); i < n; i++ {
		if len(data) < pos+2 {
			return nil, errParsePackCorrupt
		}
		keyLen := int(binary.LittleEndian.Uint16(data[pos:]))
		pos += 2
		if len(data) < pos+keyLen+parsePackEntryFixed-2 {
			return nil, errParsePackCorrupt
		}
		key := string(data[pos : pos+keyLen])
		pos += keyLen
		off := binary.LittleEndian.Uint64(data[pos:])
		pos += 8
		ln := binary.LittleEndian.Uint64(data[pos:])
		pos += 8
		crc := binary.LittleEndian.Uint32(data[pos:])
		pos += 4
		if off+ln > uint64(len(data)) {
			return nil, errParsePackCorrupt
		}
		idx[key] = parsePackEntry{offset: off, length: ln, crc: crc}
	}
	return idx, nil
}

func (h *parsePackHandle) get(hash string) ([]byte, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if !h.loaded || h.missing || h.index == nil {
		return nil, false
	}
	e, ok := h.index[hash]
	if !ok {
		return nil, false
	}
	blob := h.data[e.offset : e.offset+e.length]
	if crc32.ChecksumIEEE(blob) != e.crc {
		return nil, false
	}
	return blob, true
}

func (ps *parsePackStore) LoadEntry(hash string) (*parseCacheEntry, error) {
	if ps == nil {
		return nil, nil
	}
	h, err := ps.packForHash(hash)
	if err != nil || h == nil {
		return nil, err
	}
	if err := h.ensureLoaded(); err != nil {
		return nil, err
	}
	data, ok := h.get(hash)
	if !ok {
		return nil, nil
	}
	var entry parseCacheEntry
	if err := cacheutil.DecodeZstdGob(bytes.NewReader(data), &entry); err != nil {
		return nil, errParsePackCorrupt
	}
	return &entry, nil
}

func (ps *parsePackStore) SaveEncodedEntries(writes []parseEncodedEntryWrite) (int, error) {
	if ps == nil || len(writes) == 0 {
		return 0, nil
	}
	grouped := make(map[*parsePackHandle][]parseEncodedEntryWrite, min(len(writes), parsePackBucketCount))
	for _, w := range writes {
		if w.hash == "" || w.data == nil {
			continue
		}
		h, err := ps.packForHash(w.hash)
		if err != nil {
			return 0, err
		}
		grouped[h] = append(grouped[h], w)
	}
	if len(grouped) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(ps.dir, 0o755); err != nil {
		return 0, err
	}

	type packWriteJob struct {
		handle *parsePackHandle
		writes []parseEncodedEntryWrite
	}
	jobs := make(chan packWriteJob)
	errCh := make(chan error, 1)
	workers := min(runtime.GOMAXPROCS(0), len(grouped))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := job.handle.putMany(job.writes); err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}()
	}
	for h, packWrites := range grouped {
		select {
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return 0, err
		case jobs <- packWriteJob{handle: h, writes: packWrites}:
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errCh:
		return 0, err
	default:
	}
	return len(grouped), nil
}

func (h *parsePackHandle) putMany(writes []parseEncodedEntryWrite) error {
	if len(writes) == 0 {
		return nil
	}
	lock := parsePackFileLock(h.path)
	lock.Lock()
	defer lock.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	var existingData []byte
	existingIndex := map[string]parsePackEntry{}
	data, err := os.ReadFile(h.path)
	switch {
	case err == nil:
		if idx, perr := parseParsePackHeader(data); perr == nil {
			existingData = data
			existingIndex = idx
		}
	case os.IsNotExist(err):
	default:
		return err
	}

	latest := make(map[string][]byte, len(writes))
	for _, w := range writes {
		if w.hash == "" || w.data == nil {
			continue
		}
		latest[w.hash] = w.data
	}
	if len(latest) == 0 {
		return nil
	}

	items := make([]parsePackItem, 0, len(existingIndex)+len(latest))
	for key, e := range existingIndex {
		if _, replacing := latest[key]; replacing {
			continue
		}
		blob := existingData[e.offset : e.offset+e.length]
		if crc32.ChecksumIEEE(blob) != e.crc {
			continue
		}
		cp := make([]byte, len(blob))
		copy(cp, blob)
		items = append(items, parsePackItem{key: key, data: cp, crc: e.crc})
	}
	for key, blob := range latest {
		items = append(items, parsePackItem{key: key, data: blob, crc: crc32.ChecksumIEEE(blob)})
	}

	buf := buildParsePack(items)
	if err := fsutil.WriteFileAtomic(h.path, buf, 0o644); err != nil {
		return err
	}
	h.data = buf
	h.index = mustParsePackHeader(buf)
	h.loaded = true
	h.missing = false
	return nil
}

func (ps *parsePackStore) RemoveEntries(hashes []string) error {
	if ps == nil || len(hashes) == 0 {
		return nil
	}
	grouped := make(map[*parsePackHandle][]string, min(len(hashes), parsePackBucketCount))
	for _, hash := range hashes {
		if hash == "" {
			continue
		}
		h, err := ps.packForHash(hash)
		if err != nil {
			return err
		}
		grouped[h] = append(grouped[h], hash)
	}
	for h, packHashes := range grouped {
		if err := h.removeMany(packHashes); err != nil {
			return err
		}
	}
	return nil
}

func (h *parsePackHandle) removeMany(hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}
	lock := parsePackFileLock(h.path)
	lock.Lock()
	defer lock.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			h.loaded = true
			h.missing = true
			h.index = nil
			h.data = nil
			return nil
		}
		return err
	}
	existingIndex, err := parseParsePackHeader(data)
	if err != nil {
		_ = os.Remove(h.path)
		h.loaded = true
		h.missing = true
		h.index = nil
		h.data = nil
		return nil
	}

	remove := make(map[string]struct{}, len(hashes))
	for _, hash := range hashes {
		remove[hash] = struct{}{}
	}
	items := make([]parsePackItem, 0, len(existingIndex))
	for key, e := range existingIndex {
		if _, ok := remove[key]; ok {
			continue
		}
		blob := data[e.offset : e.offset+e.length]
		if crc32.ChecksumIEEE(blob) != e.crc {
			continue
		}
		cp := make([]byte, len(blob))
		copy(cp, blob)
		items = append(items, parsePackItem{key: key, data: cp, crc: e.crc})
	}
	if len(items) == 0 {
		if err := os.Remove(h.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		h.loaded = true
		h.missing = true
		h.index = nil
		h.data = nil
		return nil
	}

	buf := buildParsePack(items)
	if err := fsutil.WriteFileAtomic(h.path, buf, 0o644); err != nil {
		return err
	}
	h.data = buf
	h.index = mustParsePackHeader(buf)
	h.loaded = true
	h.missing = false
	return nil
}

func buildParsePack(items []parsePackItem) []byte {
	headerSize := parsePackHeaderFixed
	for _, it := range items {
		headerSize += parsePackEntryFixed + len(it.key)
	}
	totalSize := headerSize
	for _, it := range items {
		totalSize += len(it.data)
	}
	buf := make([]byte, totalSize)
	binary.LittleEndian.PutUint32(buf[0:4], parsePackMagic)
	binary.LittleEndian.PutUint16(buf[4:6], parsePackVersion)
	binary.LittleEndian.PutUint32(buf[6:10], uint32(len(items)))
	pos := parsePackHeaderFixed
	dataOff := headerSize
	for _, it := range items {
		binary.LittleEndian.PutUint16(buf[pos:], uint16(len(it.key)))
		pos += 2
		copy(buf[pos:], it.key)
		pos += len(it.key)
		binary.LittleEndian.PutUint64(buf[pos:], uint64(dataOff))
		pos += 8
		binary.LittleEndian.PutUint64(buf[pos:], uint64(len(it.data)))
		pos += 8
		binary.LittleEndian.PutUint32(buf[pos:], it.crc)
		pos += 4
		copy(buf[dataOff:], it.data)
		dataOff += len(it.data)
	}
	return buf
}

func mustParsePackHeader(data []byte) map[string]parsePackEntry {
	idx, err := parseParsePackHeader(data)
	if err != nil {
		panic(err)
	}
	return idx
}

func parsePackFileLock(path string) *sync.Mutex {
	v, _ := parsePackPathLocks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func countParsePackGroups(writes []parseEncodedEntryWrite) int64 {
	if len(writes) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, min(len(writes), parsePackBucketCount))
	for _, w := range writes {
		if len(w.hash) < 2 {
			continue
		}
		seen[w.hash[:2]] = struct{}{}
	}
	return int64(len(seen))
}

func (ps *parsePackStore) ScanEntries() ([]parsePackRecord, error) {
	if ps == nil {
		return nil, nil
	}
	var records []parsePackRecord
	err := filepath.Walk(ps.dir, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			if os.IsNotExist(werr) {
				return nil
			}
			return werr
		}
		if info == nil || info.IsDir() || filepath.Ext(path) != parsePackExt {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		idx, err := parseParsePackHeader(data)
		if err != nil {
			return nil
		}
		for hash, e := range idx {
			records = append(records, parsePackRecord{hash: hash, size: int64(e.length)})
		}
		return nil
	})
	if os.IsNotExist(err) {
		return records, nil
	}
	return records, err
}
