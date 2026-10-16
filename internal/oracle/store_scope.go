package oracle

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/store"
)

// StoreScope fixes the backend identity for one oracle invocation. In
// particular, queued writes retain this value if the jar changes on disk.
type StoreScope struct {
	Backend        Backend
	JarToken       string
	ClasspathToken string
	JvmTarget      string
	// KotlinSettings is the canonical identity of the Kotlin compiler settings
	// the scope's facts were computed under; empty means embedded defaults.
	KotlinSettings string
	version        [16]byte
}

// NewStoreScopeWithTarget includes the FIR compilation target in cache identity.
func NewStoreScopeWithTarget(backend Backend, jarPath string, classpath []string, jvmTarget string) StoreScope {
	return NewStoreScopeWithSettings(backend, jarPath, classpath, jvmTarget, gradlemodel.KotlinSettings{})
}

// NewStoreScopeWithSettings includes the compilation target and the project's
// effective Kotlin compiler settings in cache identity, for either backend.
// The language level, API level, opt-ins and semantic flags change the facts a
// compilation produces without changing any source or classpath entry, so a
// settings-only change must read as a different scope. Zero settings leave the
// identity exactly as it was before settings existed.
func NewStoreScopeWithSettings(backend Backend, jarPath string, classpath []string, jvmTarget string, settings gradlemodel.KotlinSettings) StoreScope {
	scope := NewStoreScope(backend, jarPath, classpath)
	targeted := backend == BackendFIR && jvmTarget != ""
	settingsID := settings.Identity()
	if !targeted && settingsID == "" {
		return scope
	}
	identity := fmt.Sprintf("oracle-v%d|%s:%s:%s", CacheVersion, backend.String(), scope.JarToken, scope.ClasspathToken)
	if targeted {
		scope.JvmTarget = jvmTarget
		identity += ":jvmTarget:" + jvmTarget
	}
	if settingsID != "" {
		scope.KotlinSettings = settingsID
		identity += "|" + settingsID
	}
	h := hashutil.HashBytes([]byte(identity))
	copy(scope.version[:], h[:])
	return scope
}

func NewStoreScope(backend Backend, jarPath string, classpath ...[]string) StoreScope {
	scope := storeScopeForToken(backend, buildid.JarToken(jarPath))
	if len(classpath) == 0 || len(classpath[0]) == 0 {
		return scope
	}
	scope.ClasspathToken = gradlemodel.ClasspathFingerprint(classpath[0])
	h := hashutil.HashBytes([]byte(fmt.Sprintf("oracle-v%d|%s:%s:%s", CacheVersion, backend.String(), scope.JarToken, scope.ClasspathToken)))
	copy(scope.version[:], h[:])
	return scope
}

func storeScopeForToken(backend Backend, jarToken string) StoreScope {
	scope := StoreScope{Backend: backend, JarToken: jarToken}
	h := hashutil.HashBytes([]byte(fmt.Sprintf("oracle-v%d|%s:%s", CacheVersion, backend.String(), jarToken)))
	copy(scope.version[:], h[:])
	return scope
}

// versionHex is the scope identity as a string, for sidecar records.
func (s StoreScope) versionHex() string { return hex.EncodeToString(s.version[:]) }

var boundStoreScopes sync.Map // *store.FileStore -> StoreScope

// BindStoreScope makes the already-resolved jar identity available to legacy
// cache APIs whose signatures receive the store but no invocation options.
// The caller must release the binding after synchronous invocation work.
func BindStoreScope(s *store.FileStore, scope StoreScope) func() {
	if s == nil {
		return func() {}
	}
	prior, hadPrior := boundStoreScopes.Load(s)
	boundStoreScopes.Store(s, scope)
	return func() {
		if hadPrior {
			boundStoreScopes.Store(s, prior)
		} else {
			boundStoreScopes.Delete(s)
		}
	}
}

func scopeForStore(s *store.FileStore, backend Backend) (StoreScope, bool) {
	if s != nil {
		if value, ok := boundStoreScopes.Load(s); ok {
			scope := value.(StoreScope)
			return scope, scope.Backend == backend
		}
	}
	return StoreScope{}, false
}

func bindFallbackStoreScope(s *store.FileStore, cacheDir, approximation string) func() {
	backend := backendForApproximation(approximation)
	if s == nil || backend == "" {
		return func() {}
	}
	if _, ok := scopeForStore(s, backend); ok {
		return func() {}
	}
	path := FindBackendJar(backend, storeScanPaths(cacheDir, ""))
	return BindStoreScope(s, NewStoreScope(backend, path))
}
