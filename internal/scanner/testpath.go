package scanner

import (
	"path/filepath"
	"strings"
	"sync"
)

var defaultTestPaths = [...]string{
	"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/jvmAndroidTest/",
	"/commonJvmTest/", "/browserCommonTest/", "/jvmCommonTest/",
	"/androidUnitTest/", "/androidInstrumentedTest/", "/jsTest/", "/iosTest/",
	"/nativeTest/", "/nonJvmCommonTest/",
	"/testShared/", "/sharedTest/",
	"/benchmark/", "/canary/",
	"/test-utils/",
	"/javatests/", "/kotlintests/", "/javatest/", "/kotlintest/",
	"/functionalTest/", "/functionaltests/",
	"/test/resources/", "/testResources/", "/testFixtures/",
	"/integration-tests/", "/integrationTest/",
	"/nonEmulatorCommonTest/", "/nonEmulatorJvmTest/",
	"/testData/", "/testdata/", "/test-data/",
	"/test/data/", "/compiler-tests/", "/compilertests/",
}

var (
	testPathMu        sync.RWMutex
	testPaths         = defaultTestPathSlice()
	testPathsOverride bool
)

func InitTestPaths(config []string, override []string) {
	next := defaultTestPathSlice()
	if len(override) > 0 {
		next = cleanTestPaths(override)
	} else if len(config) > 0 {
		next = append(next, cleanTestPaths(config)...)
	}
	testPathMu.Lock()
	testPaths = next
	testPathsOverride = len(override) > 0
	testPathMu.Unlock()
}

func IsTestFile(path string) bool {
	slash := filepath.ToSlash(path)
	testPathMu.RLock()
	defer testPathMu.RUnlock()
	for _, marker := range testPaths {
		if marker != "" && strings.Contains(slash, marker) {
			return true
		}
	}
	if testPathsOverride {
		return false
	}
	return isGradleTestSourceSet(slash)
}

func isGradleTestSourceSet(slash string) bool {
	const sourceSetPrefix = "/src/"
	if strings.HasPrefix(slash, "src/") {
		segment := slash[len("src/"):]
		if end := strings.IndexByte(segment, '/'); end >= 0 {
			segment = segment[:end]
		}
		if segment == "test" || strings.HasSuffix(segment, "Test") {
			return true
		}
	}
	for offset := 0; ; {
		index := strings.Index(slash[offset:], sourceSetPrefix)
		if index < 0 {
			return false
		}
		start := offset + index + len(sourceSetPrefix)
		end := strings.IndexByte(slash[start:], '/')
		if end < 0 {
			end = len(slash)
		} else {
			end += start
		}
		segment := slash[start:end]
		if segment == "test" || strings.HasSuffix(segment, "Test") {
			return true
		}
		offset = start
	}
}

func defaultTestPathSlice() []string {
	out := make([]string, len(defaultTestPaths))
	copy(out, defaultTestPaths[:])
	return out
}

func cleanTestPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}
