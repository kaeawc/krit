package oracle

import (
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
	version        [16]byte
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
