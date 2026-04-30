package librarymodel

import (
	"strconv"
	"strings"
)

type JavaKnownType struct {
	FQN        string
	Supertypes []string
}

type JavaTypeProfile struct {
	KnownTypes        map[string]JavaKnownType
	MethodReturns     map[string]string
	AnnotationAliases map[string]string
}

func DefaultJavaTypeProfile() JavaTypeProfile {
	return javaTypeProfile(true)
}

func javaTypeProfile(includeRoom bool) JavaTypeProfile {
	profile := JavaTypeProfile{
		KnownTypes:        make(map[string]JavaKnownType),
		MethodReturns:     make(map[string]string),
		AnnotationAliases: make(map[string]string),
	}
	profile.addType("android.webkit.WebView")
	profile.addType("android.webkit.WebSettings")
	profile.addType("android.content.SharedPreferences")
	profile.addType("android.content.SharedPreferences.Editor")
	profile.addType("android.app.FragmentManager")
	profile.addType("android.app.FragmentTransaction")
	profile.addType("androidx.fragment.app.FragmentManager")
	profile.addType("androidx.fragment.app.FragmentTransaction")
	profile.addType("android.view.View")
	profile.addType("android.view.ViewPropertyAnimator")
	profile.addType("android.os.Handler")
	profile.addType("android.os.Looper")
	profile.addType("android.database.Cursor")
	profile.addType("java.lang.Runnable")
	profile.addType("java.lang.String")
	profile.addType("java.util.concurrent.Executor")
	profile.addType("java.util.concurrent.ExecutorService", "java.util.concurrent.Executor")
	profile.addType("java.util.concurrent.ScheduledExecutorService", "java.util.concurrent.ExecutorService", "java.util.concurrent.Executor")

	profile.addMethodReturn("android.webkit.WebView", "getSettings", 0, "android.webkit.WebSettings")
	profile.addMethodReturn("android.content.SharedPreferences", "edit", 0, "android.content.SharedPreferences.Editor")
	profile.addMethodReturn("android.app.FragmentManager", "beginTransaction", 0, "android.app.FragmentTransaction")
	profile.addMethodReturn("androidx.fragment.app.FragmentManager", "beginTransaction", 0, "androidx.fragment.app.FragmentTransaction")
	profile.addMethodReturn("java.lang.String", "format", -1, "java.lang.String")
	profile.addMethodReturn("java.lang.String", "trim", 0, "java.lang.String")
	profile.addMethodReturn("java.lang.String", "replace", -1, "java.lang.String")
	profile.addMethodReturn("android.view.View", "animate", 0, "android.view.ViewPropertyAnimator")

	profile.addAnnotationAlias("androidx.annotation.CheckResult", "CheckResult")
	profile.addAnnotationAlias("com.google.errorprone.annotations.CheckReturnValue", "CheckResult")
	profile.addAnnotationAlias("javax.annotation.CheckReturnValue", "CheckResult")

	if includeRoom {
		profile.addType("androidx.room.Dao")
		profile.addType("androidx.room.Query")
		profile.addType("androidx.room.Insert")
		profile.addType("androidx.room.Update")
		profile.addType("androidx.room.Delete")
		profile.addType("androidx.room.Transaction")
		profile.addAnnotationAlias("androidx.room.Dao", "RoomDao")
		profile.addAnnotationAlias("androidx.room.Query", "RoomOperation")
		profile.addAnnotationAlias("androidx.room.Insert", "RoomOperation")
		profile.addAnnotationAlias("androidx.room.Update", "RoomOperation")
		profile.addAnnotationAlias("androidx.room.Delete", "RoomOperation")
		profile.addAnnotationAlias("androidx.room.Transaction", "RoomOperation")
	}
	return profile
}

func (p *JavaTypeProfile) addType(fqn string, supertypes ...string) {
	if p.KnownTypes == nil {
		p.KnownTypes = make(map[string]JavaKnownType)
	}
	p.KnownTypes[javaProfileNormalize(fqn)] = JavaKnownType{
		FQN:        javaProfileNormalize(fqn),
		Supertypes: javaProfileNormalizeList(supertypes),
	}
}

func (p *JavaTypeProfile) addMethodReturn(owner, method string, arity int, returnType string) {
	if p.MethodReturns == nil {
		p.MethodReturns = make(map[string]string)
	}
	p.MethodReturns[javaMethodKey(owner, method, arity)] = javaProfileNormalize(returnType)
}

func (p *JavaTypeProfile) addAnnotationAlias(annotation, alias string) {
	if p.AnnotationAliases == nil {
		p.AnnotationAliases = make(map[string]string)
	}
	p.AnnotationAliases[javaProfileNormalize(annotation)] = javaProfileNormalize(alias)
}

func (p JavaTypeProfile) IsKnownFrameworkType(candidate string) bool {
	candidate = javaProfileNormalize(candidate)
	if candidate == "" {
		return false
	}
	if _, ok := p.KnownTypes[candidate]; ok {
		return true
	}
	for fqn := range p.KnownTypes {
		if javaTypeNameMatches(candidate, fqn) {
			return true
		}
	}
	return false
}

func (p JavaTypeProfile) IsSubtypeCandidate(supertype, candidate string) bool {
	supertype = javaProfileNormalize(supertype)
	candidate = javaProfileNormalize(candidate)
	if supertype == "" || candidate == "" {
		return false
	}
	if javaTypeNameMatches(candidate, supertype) {
		return true
	}
	typ, ok := p.KnownTypes[candidate]
	if !ok {
		for fqn, known := range p.KnownTypes {
			if javaTypeNameMatches(candidate, fqn) {
				typ = known
				ok = true
				break
			}
		}
	}
	if !ok {
		return false
	}
	for _, parent := range typ.Supertypes {
		if javaTypeNameMatches(parent, supertype) || p.IsSubtypeCandidate(supertype, parent) {
			return true
		}
	}
	return false
}

func (p JavaTypeProfile) MethodReturn(owner, method string, arity int) string {
	owner = javaProfileNormalize(owner)
	method = strings.TrimSpace(method)
	if owner == "" || method == "" {
		return ""
	}
	for _, key := range []string{javaMethodKey(owner, method, arity), javaMethodKey(owner, method, -1)} {
		if ret := p.MethodReturns[key]; ret != "" {
			return ret
		}
	}
	for fqn := range p.KnownTypes {
		if !javaTypeNameMatches(owner, fqn) {
			continue
		}
		for _, key := range []string{javaMethodKey(fqn, method, arity), javaMethodKey(fqn, method, -1)} {
			if ret := p.MethodReturns[key]; ret != "" {
				return ret
			}
		}
	}
	return ""
}

func (p JavaTypeProfile) AnnotationImplies(annotation, implied string) bool {
	annotation = javaProfileNormalize(annotation)
	implied = javaProfileNormalize(implied)
	if annotation == "" || implied == "" {
		return false
	}
	return javaTypeNameMatches(annotation, implied) || javaTypeNameMatches(p.AnnotationAliases[annotation], implied)
}

func javaMethodKey(owner, method string, arity int) string {
	return javaProfileNormalize(owner) + "#" + strings.TrimSpace(method) + "/" + strconv.Itoa(arity)
}

func javaProfileNormalize(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "$", "."))
	if cut := strings.Index(value, "("); cut >= 0 {
		value = value[:cut]
	}
	return value
}

func javaProfileNormalizeList(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if normalized := javaProfileNormalize(value); normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func javaTypeNameMatches(got, want string) bool {
	got = javaProfileNormalize(got)
	want = javaProfileNormalize(want)
	return got != "" && want != "" && (got == want || strings.HasSuffix(got, "."+want) || strings.HasSuffix(want, "."+got))
}
