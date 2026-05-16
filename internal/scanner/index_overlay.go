package scanner

import (
	"sort"

	"github.com/kaeawc/krit/internal/perf"
)

const (
	crossFileOverlayMaxChanged = 64
	crossFileOverlayMaxEntries = 512
)

func crossFileEntryMap(entries []fingerprintEntry) map[string]string {
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		out[e.Path] = e.Hash
	}
	return out
}

func diffCrossFileEntryPaths(oldEntries, newEntries []fingerprintEntry) (map[string]bool, map[string]bool) {
	oldMap := crossFileEntryMap(oldEntries)
	newMap := crossFileEntryMap(newEntries)
	removePaths := make(map[string]bool)
	addPaths := make(map[string]bool)
	for path, oldHash := range oldMap {
		if newHash, ok := newMap[path]; !ok || newHash != oldHash {
			removePaths[path] = true
		}
	}
	for path, newHash := range newMap {
		if oldHash, ok := oldMap[path]; !ok || oldHash != newHash {
			addPaths[path] = true
		}
	}
	return removePaths, addPaths
}

func selectFilesByPath[T interface{ *File | *xmlCacheFile }](files []T, paths map[string]bool) []T {
	if len(paths) == 0 {
		return nil
	}
	out := make([]T, 0, min(len(files), len(paths)))
	for _, f := range files {
		if f == nil {
			continue
		}
		var path string
		switch v := any(f).(type) {
		case *File:
			path = v.Path
		case *xmlCacheFile:
			path = v.Path
		}
		if paths[path] {
			out = append(out, f)
		}
	}
	return out
}

func overlayMetaForEntries(prev CrossFileCacheMeta, current []fingerprintEntry) CrossFileCacheMeta {
	payloadEntries := append([]fingerprintEntry(nil), crossFilePayloadEntries(prev)...)
	if len(payloadEntries) == 0 {
		payloadEntries = append([]fingerprintEntry(nil), prev.Entries...)
	}
	removeFromPayload, addToPayload := diffCrossFileEntryPaths(payloadEntries, current)
	overlayEntries := make([]fingerprintEntry, 0, len(addToPayload))
	for _, e := range current {
		if addToPayload[e.Path] {
			overlayEntries = append(overlayEntries, e)
		}
	}
	removedPaths := make([]string, 0, len(removeFromPayload))
	for path := range removeFromPayload {
		removedPaths = append(removedPaths, path)
	}
	sort.Strings(removedPaths)
	return CrossFileCacheMeta{
		Entries:             append([]fingerprintEntry(nil), current...),
		PayloadEntries:      payloadEntries,
		OverlayEntries:      overlayEntries,
		RemovedPayloadPaths: removedPaths,
	}
}

func shouldUseCrossFileOverlay(prev CrossFileCacheMeta, current []fingerprintEntry, removePaths, addPaths map[string]bool) bool {
	changed := len(removePaths)
	if len(addPaths) > changed {
		changed = len(addPaths)
	}
	if changed == 0 || changed > crossFileOverlayMaxChanged {
		return false
	}
	meta := overlayMetaForEntries(prev, current)
	return len(meta.OverlayEntries)+len(meta.RemovedPayloadPaths) <= crossFileOverlayMaxEntries
}

func buildIndexFromPriorOverlay(cacheDir string, entries []fingerprintEntry, files []*File, javaFiles []*File, xmlFiles []*xmlCacheFile, workers int, priorLoader PriorIndexLoader, tracker perf.Tracker) (*CodeIndex, bool) {
	var priorIdx *CodeIndex
	var priorMeta CrossFileCacheMeta
	var ok bool
	var loaderHit bool
	if priorLoader != nil {
		// Daemon fast path: skip the ~2.6 s gob decode of the prior
		// payload when the same process built it last analyze. The
		// loader returns the resident pointer; BuildIndexIncremental
		// mutates it in place — the daemon's per-project mutex
		// serializes us with any other reader.
		if loaded, loadedMeta, loaderOk := priorLoader(); loaderOk && loaded != nil && len(loadedMeta.Entries) > 0 {
			priorIdx, priorMeta, ok, loaderHit = loaded, loadedMeta, true, true
		}
	}
	if !ok {
		priorIdx, priorMeta, ok = LoadCurrentCrossFileCacheIndex(cacheDir)
	}
	if !ok || len(priorMeta.Entries) == 0 {
		return nil, false
	}
	removePaths, addPaths := diffCrossFileEntryPaths(priorMeta.Entries, entries)
	if !shouldUseCrossFileOverlay(priorMeta, entries, removePaths, addPaths) {
		return nil, false
	}

	changedKotlin := selectFilesByPath(files, addPaths)
	changedJava := selectFilesByPath(javaFiles, addPaths)
	changedXML := selectFilesByPath(xmlFiles, addPaths)
	symbols, refs, _ := collectIndexDataSharded(cacheDir, changedKotlin, changedJava, changedXML, workers, tracker)
	idx := BuildIndexIncremental(priorIdx, removePaths, symbols, refs)

	meta := overlayMetaForEntries(priorMeta, entries)
	meta.KotlinFiles = len(files)
	meta.JavaFiles = len(javaFiles)
	meta.XMLFiles = len(xmlFiles)
	// Overlay entries are kept on disk so a future process can also
	// reuse them. When we came from a resident-prior loader, the disk
	// overlay state may not include the overlay entries we need; in
	// that case fall back to a fresh full build by reporting miss.
	if !loaderHit {
		if _, _, overlayOk := loadCrossFileOverlayEntries(cacheDir, meta.OverlayEntries); !overlayOk {
			return nil, false
		}
	}
	if err := SaveCrossFileCacheOverlay(cacheDir, fingerprintCrossFileEntries(entries), meta, idx); err != nil {
		return nil, false
	}
	return idx, true
}
