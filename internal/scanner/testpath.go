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
	testPathMu sync.RWMutex
	testPaths  = defaultTestPathSlice()
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
	return false
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
