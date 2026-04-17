package di

import "strings"

// Scope describes a lifecycle scope annotation attached to a DI binding.
type Scope struct {
	Name  string
	Rank  int
	Known bool
}

// IsSet reports whether a scope annotation was detected, even if Krit does not
// yet know how to order that scope relative to other lifecycles.
func (s Scope) IsSet() bool {
	return s.Name != ""
}

// WiderThan reports whether this scope is definitely wider-lived than other.
func (s Scope) WiderThan(other Scope) bool {
	return s.Known && other.Known && s.Rank > other.Rank
}

var scopeRanks = map[string]int{
	"Singleton":              100,
	"AppScope":               100,
	"ActivityRetainedScoped": 80,
	"ViewModelScoped":        70,
	"ActivityScoped":         60,
	"ServiceScoped":          60,
	"FragmentScoped":         50,
	"ViewScoped":             40,
}

// ResolveScope converts an annotation name into an ordered lifecycle scope when
// Krit knows the lifecycle. Unknown scope-like annotations are preserved for
// future graph analysis but remain unordered for now.
func ResolveScope(annotation string) Scope {
	name := simpleName(annotation)
	if name == "" {
		return Scope{}
	}
	if rank, ok := scopeRanks[name]; ok {
		return Scope{Name: name, Rank: rank, Known: true}
	}
	if looksLikeScopeAnnotation(name) {
		return Scope{Name: name}
	}
	return Scope{}
}

func looksLikeScopeAnnotation(name string) bool {
	return name == "Singleton" ||
		strings.HasSuffix(name, "Scoped") ||
		strings.HasSuffix(name, "Scope")
}

func simpleName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}
