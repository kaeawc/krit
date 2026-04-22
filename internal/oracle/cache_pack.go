package oracle

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

const (
	oraclePackMagic       uint32 = 0x4b4f504b // "KOPK"
	oraclePackVersion     uint16 = 1
	oraclePackSubdir             = "packs-v1"
	oraclePackBucketCount        = 256
	oraclePackHeaderFixed        = 4 + 2 + 4     // magic + version + entryCount
	oraclePackEntryFixed         = 2 + 8 + 8 + 4 // keyLen + offset + length + crc32
	oraclePackExt                = ".pack"
)

var (
	errOraclePackCorrupt = errors.New("oracle pack corrupt")
	oraclePackPathLocks  sync.Map
)

type oraclePackEntry struct {
	offset uint64
	length uint64
	crc    uint32
}

type oracleEncodedEntryWrite struct {
	hash string
	data []byte
}

type oraclePackItem struct {
	key  string
	data []byte
	crc  uint32
}

type oraclePackHandle struct {
	path    string
	mu      sync.RWMutex
	loaded  bool
	missing bool
	index   map[string]oraclePackEntry
	data    []byte
}

type oraclePackStore struct {
	dir   string
	packs [oraclePackBucketCount]*oraclePackHandle
}

func newOraclePackStore(cacheDir string) *oraclePackStore {
	if cacheDir == "" {
		return nil
	}
	ps := &oraclePackStore{dir: filepath.Join(cacheDir, oraclePackSubdir)}
	for i := 0; i < oraclePackBucketCount; i++ {
		ps.packs[i] = &oraclePackHandle{
			path: filepath.Join(ps.dir, fmt.Sprintf("%02x%s", i, oraclePackExt)),
		}
	}
	return ps
}

func (ps *oraclePackStore) packForHash(hash string) (*oraclePackHandle, error) {
	if ps == nil {
		return nil, nil
	}
	if len(hash) < 2 {
		return nil, fmt.Errorf("oracle cache hash too short: %q", hash)
	}
	n, err := strconv.ParseUint(hash[:2], 16, 8)
	if err != nil {
		return nil, fmt.Errorf("oracle cache hash not hex: %q", hash)
	}
	return ps.packs[n], nil
}

func (h *oraclePackHandle) ensureLoaded() error {
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
	idx, err := parseOraclePackHeader(data)
	if err != nil {
		return err
	}
	h.data = data
	h.index = idx
	h.loaded = true
	h.missing = false
	return nil
}

func parseOraclePackHeader(data []byte) (map[string]oraclePackEntry, error) {
	if len(data) < oraclePackHeaderFixed {
		return nil, errOraclePackCorrupt
	}
	if binary.LittleEndian.Uint32(data[0:4]) != oraclePackMagic {
		return nil, errOraclePackCorrupt
	}
	if binary.LittleEndian.Uint16(data[4:6]) != oraclePackVersion {
		return nil, errOraclePackCorrupt
	}
	n := binary.LittleEndian.Uint32(data[6:10])
	idx := make(map[string]oraclePackEntry, n)
	pos := oraclePackHeaderFixed
	for i := uint32(0); i < n; i++ {
		if len(data) < pos+2 {
			return nil, errOraclePackCorrupt
		}
		keyLen := int(binary.LittleEndian.Uint16(data[pos:]))
		pos += 2
		if len(data) < pos+keyLen+oraclePackEntryFixed-2 {
			return nil, errOraclePackCorrupt
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
			return nil, errOraclePackCorrupt
		}
		idx[key] = oraclePackEntry{offset: off, length: ln, crc: crc}
	}
	return idx, nil
}

func (h *oraclePackHandle) get(hash string) ([]byte, bool) {
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

func (ps *oraclePackStore) LoadEntry(hash string) (*CacheEntry, error) {
	if ps == nil {
		return nil, nil
	}
	h, err := ps.packForHash(hash)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, nil
	}
	if err := h.ensureLoaded(); err != nil {
		return nil, err
	}
	data, ok := h.get(hash)
	if !ok {
		return nil, nil
	}
	entry, err := parseCacheEntryData(data, h.path)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (ps *oraclePackStore) SaveEncodedEntries(writes []oracleEncodedEntryWrite) (int, error) {
	if ps == nil || len(writes) == 0 {
		return 0, nil
	}
	grouped := make(map[*oraclePackHandle][]oracleEncodedEntryWrite, min(len(writes), oraclePackBucketCount))
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
		handle *oraclePackHandle
		writes []oracleEncodedEntryWrite
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

func (h *oraclePackHandle) putMany(writes []oracleEncodedEntryWrite) error {
	if len(writes) == 0 {
		return nil
	}
	lock := oraclePackFileLock(h.path)
	lock.Lock()
	defer lock.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	var existingData []byte
	existingIndex := map[string]oraclePackEntry{}
	data, err := os.ReadFile(h.path)
	switch {
	case err == nil:
		idx, perr := parseOraclePackHeader(data)
		if perr == nil {
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

	items := make([]oraclePackItem, 0, len(existingIndex)+len(latest))
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
		items = append(items, oraclePackItem{key: key, data: cp, crc: e.crc})
	}
	for key, blob := range latest {
		items = append(items, oraclePackItem{key: key, data: blob, crc: crc32.ChecksumIEEE(blob)})
	}

	buf := buildOraclePack(items)
	if err := fsutil.WriteFileAtomic(h.path, buf, 0o644); err != nil {
		return err
	}
	h.data = buf
	h.index = mustParseOraclePackHeader(buf)
	h.loaded = true
	h.missing = false
	return nil
}

func buildOraclePack(items []oraclePackItem) []byte {
	headerSize := oraclePackHeaderFixed
	for _, it := range items {
		headerSize += oraclePackEntryFixed + len(it.key)
	}
	totalSize := headerSize
	for _, it := range items {
		totalSize += len(it.data)
	}
	buf := make([]byte, totalSize)
	binary.LittleEndian.PutUint32(buf[0:4], oraclePackMagic)
	binary.LittleEndian.PutUint16(buf[4:6], oraclePackVersion)
	binary.LittleEndian.PutUint32(buf[6:10], uint32(len(items)))
	pos := oraclePackHeaderFixed
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

func mustParseOraclePackHeader(data []byte) map[string]oraclePackEntry {
	idx, err := parseOraclePackHeader(data)
	if err != nil {
		panic(err)
	}
	return idx
}

func oraclePackFileLock(path string) *sync.Mutex {
	v, _ := oraclePackPathLocks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func countOraclePackGroups(writes []oracleEncodedEntryWrite) int64 {
	if len(writes) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, min(len(writes), oraclePackBucketCount))
	for _, w := range writes {
		if len(w.hash) < 2 {
			continue
		}
		seen[w.hash[:2]] = struct{}{}
	}
	return int64(len(seen))
}

func writeEntriesData(cacheDir string, writes []oracleEncodedEntryWrite) error {
	if len(writes) == 0 {
		return nil
	}
	ps := newOraclePackStore(cacheDir)
	if _, err := ps.SaveEncodedEntries(writes); err != nil {
		return fmt.Errorf("write oracle packs: %w", err)
	}
	recordOracleDir(cacheDir)
	oracleCacheWrites.Add(int64(len(writes)))
	oracleCacheLastWrite.Store(time.Now().Unix())
	return nil
}

func oraclePackStats(cacheDir string) (count int, bytes int64, err error) {
	root := filepath.Join(cacheDir, oraclePackSubdir)
	err = filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			if os.IsNotExist(werr) {
				return nil
			}
			return werr
		}
		if info.IsDir() || filepath.Ext(info.Name()) != oraclePackExt {
			return nil
		}
		bytes += info.Size()
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		idx, parseErr := parseOraclePackHeader(data)
		if parseErr != nil {
			return nil
		}
		count += len(idx)
		return nil
	})
	return count, bytes, err
}
