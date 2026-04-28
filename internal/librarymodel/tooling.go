package librarymodel

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	LatestStableKotlinVersion   = "2.2"
	LatestStableKSPVersion      = "2.2"
	LatestStableAGPVersion      = "8.13"
	LatestStableCompileSDK      = "36"
	LatestStableTargetSDK       = "36"
	LatestStableJavaVersion     = "21"
	LatestStableKotlinJvmTarget = "21"
)

type Presence int

const (
	PresenceUnknown Presence = iota
	PresenceAbsent
	PresencePresent
)

type VersionPolicy int

const (
	VersionUnknown VersionPolicy = iota
	VersionKnown
	VersionAssumeLatestStable
)

type VersionedPresence struct {
	Presence      Presence
	Version       string
	VersionPolicy VersionPolicy
}

func KnownVersion(version string) VersionedPresence {
	return VersionedPresence{Presence: PresencePresent, Version: version, VersionPolicy: VersionKnown}
}

func PresentAssumeLatestStable() VersionedPresence {
	return VersionedPresence{Presence: PresencePresent, VersionPolicy: VersionAssumeLatestStable}
}

func (v VersionedPresence) EffectiveVersion(latestStable string) string {
	if v.Presence != PresencePresent {
		return ""
	}
	if v.Version != "" {
		return v.Version
	}
	if v.VersionPolicy == VersionAssumeLatestStable {
		return latestStable
	}
	return ""
}

func mergeVersionedPresence(current, next VersionedPresence) VersionedPresence {
	if next.Presence == PresenceUnknown {
		return current
	}
	if current.Presence == PresenceUnknown {
		return next
	}
	if current.Presence != PresencePresent {
		return next
	}
	if next.Presence != PresencePresent {
		return current
	}
	if current.Version == "" && next.Version != "" {
		return next
	}
	if current.VersionPolicy != VersionKnown && next.VersionPolicy == VersionKnown {
		return next
	}
	return current
}

type KotlinProfile struct {
	Compiler        VersionedPresence
	LanguageVersion VersionedPresence
	ApiVersion      VersionedPresence
	K2              Presence
}

func (k KotlinProfile) EffectiveCompilerVersion() string {
	return k.Compiler.EffectiveVersion(LatestStableKotlinVersion)
}

func (k KotlinProfile) EffectiveLanguageVersion() string {
	if version := k.LanguageVersion.EffectiveVersion(LatestStableKotlinVersion); version != "" {
		return version
	}
	return k.EffectiveCompilerVersion()
}

func (k KotlinProfile) EffectiveApiVersion() string {
	if version := k.ApiVersion.EffectiveVersion(LatestStableKotlinVersion); version != "" {
		return version
	}
	return k.EffectiveLanguageVersion()
}

type KSPProfile struct {
	Tool       VersionedPresence
	Processors []Dependency
}

func (k KSPProfile) EffectiveVersion() string {
	return k.Tool.EffectiveVersion(LatestStableKSPVersion)
}

type AndroidProfile struct {
	AGP        VersionedPresence
	CompileSDK VersionedPresence
	TargetSDK  VersionedPresence
	MinSDK     VersionedPresence
}

func (a AndroidProfile) EffectiveAGPVersion() string {
	return a.AGP.EffectiveVersion(LatestStableAGPVersion)
}

func (a AndroidProfile) EffectiveCompileSDK() string {
	return a.CompileSDK.EffectiveVersion(LatestStableCompileSDK)
}

func (a AndroidProfile) EffectiveTargetSDK() string {
	return a.TargetSDK.EffectiveVersion(LatestStableTargetSDK)
}

func (a AndroidProfile) EffectiveMinSDK() string {
	return a.MinSDK.EffectiveVersion("")
}

type JVMProfile struct {
	ToolchainLanguageVersion VersionedPresence
	SourceCompatibility      VersionedPresence
	TargetCompatibility      VersionedPresence
	KotlinJvmTarget          VersionedPresence
}

func (j JVMProfile) EffectiveTargetBytecode() string {
	if version := j.KotlinJvmTarget.EffectiveVersion(LatestStableKotlinJvmTarget); version != "" {
		return version
	}
	if version := j.TargetCompatibility.EffectiveVersion(LatestStableJavaVersion); version != "" {
		return version
	}
	if version := j.ToolchainLanguageVersion.EffectiveVersion(LatestStableJavaVersion); version != "" {
		return version
	}
	return ""
}

var (
	kotlinPluginVersionRe = regexp.MustCompile(`(?:id\s*\(\s*["']org\.jetbrains\.kotlin\.[^"']+["']\s*\)|kotlin\s*\(\s*["'][^"']+["']\s*\))\s*version\s*["']([^"']+)["']`)
	kotlinPluginRe        = regexp.MustCompile(`(?:id\s*\(\s*["']org\.jetbrains\.kotlin\.[^"']+["']\s*\)|kotlin\s*\(\s*["'][^"']+["']\s*\)|alias\s*\(\s*libs\.plugins\.[^)]*kotlin[^)]*\))`)
	kotlinLanguageRe      = regexp.MustCompile(`languageVersion(?:\.set)?\s*(?:=|\()?\s*(?:KotlinVersion\.)?(?:KOTLIN_)?["']?([0-9]+(?:[._][0-9]+)?)["']?`)
	kotlinAPIRe           = regexp.MustCompile(`apiVersion(?:\.set)?\s*(?:=|\()?\s*(?:KotlinVersion\.)?(?:KOTLIN_)?["']?([0-9]+(?:[._][0-9]+)?)["']?`)
	kotlinK2Re            = regexp.MustCompile(`(?:languageVersion(?:\.set)?\s*(?:=|\()?\s*(?:KotlinVersion\.)?KOTLIN_2|freeCompilerArgs[^\n]*(?:-Xuse-k2|-language-version\s+2\.))`)

	kspPluginVersionRe = regexp.MustCompile(`id\s*\(\s*["']com\.google\.devtools\.ksp["']\s*\)\s*version\s*["']([^"']+)["']`)
	kspPluginRe        = regexp.MustCompile(`(?:id\s*\(\s*["']com\.google\.devtools\.ksp["']\s*\)|alias\s*\(\s*libs\.plugins\.[^)]*ksp[^)]*\)|\bksp\s*\()`)

	agpPluginVersionRe = regexp.MustCompile(`id\s*\(\s*["']com\.android\.[^"']+["']\s*\)\s*version\s*["']([^"']+)["']`)
	androidPluginRe    = regexp.MustCompile(`(?:id\s*\(\s*["']com\.android\.[^"']+["']\s*\)|alias\s*\(\s*libs\.plugins\.[^)]*android[^)]*\))`)

	javaToolchainRe       = regexp.MustCompile(`(?:jvmToolchain\s*\(?\s*|JavaLanguageVersion\.of\s*\(\s*)(\d+)`)
	sourceCompatibilityRe = regexp.MustCompile(`sourceCompatibility\s*=?\s*(?:JavaVersion\.)?(?:VERSION_)?["']?([0-9]+(?:_[0-9]+)?)["']?`)
	targetCompatibilityRe = regexp.MustCompile(`targetCompatibility\s*=?\s*(?:JavaVersion\.)?(?:VERSION_)?["']?([0-9]+(?:_[0-9]+)?)["']?`)
	kotlinJvmTargetRe     = regexp.MustCompile(`jvmTarget(?:\.set)?\s*(?:=|\()?\s*(?:JvmTarget\.)?(?:JVM_)?["']?([0-9]+(?:_[0-9]+)?)["']?`)
)

func extractToolingProfile(content string, cfgMinSDK, cfgTargetSDK, cfgCompileSDK int) (KotlinProfile, KSPProfile, AndroidProfile, JVMProfile) {
	var kotlin KotlinProfile
	var ksp KSPProfile
	var android AndroidProfile
	var jvm JVMProfile

	if version := firstMatch(kotlinPluginVersionRe, content); version != "" {
		kotlin.Compiler = KnownVersion(normalizeVersion(version))
	} else if kotlinPluginRe.MatchString(content) {
		kotlin.Compiler = PresentAssumeLatestStable()
	}
	if version := firstMatch(kotlinLanguageRe, content); version != "" {
		kotlin.LanguageVersion = KnownVersion(normalizeVersion(version))
	}
	if version := firstMatch(kotlinAPIRe, content); version != "" {
		kotlin.ApiVersion = KnownVersion(normalizeVersion(version))
	}
	if kotlinK2Re.MatchString(content) {
		kotlin.K2 = PresencePresent
	}

	if version := firstMatch(kspPluginVersionRe, content); version != "" {
		ksp.Tool = KnownVersion(normalizeVersion(version))
	} else if kspPluginRe.MatchString(content) {
		ksp.Tool = PresentAssumeLatestStable()
	}

	if version := firstMatch(agpPluginVersionRe, content); version != "" {
		android.AGP = KnownVersion(normalizeVersion(version))
	} else if androidPluginRe.MatchString(content) {
		android.AGP = PresentAssumeLatestStable()
	}
	if cfgCompileSDK > 0 {
		android.CompileSDK = KnownVersion(strconv.Itoa(cfgCompileSDK))
	} else if android.AGP.Presence == PresencePresent {
		android.CompileSDK = PresentAssumeLatestStable()
	}
	if cfgTargetSDK > 0 {
		android.TargetSDK = KnownVersion(strconv.Itoa(cfgTargetSDK))
	} else if android.AGP.Presence == PresencePresent {
		android.TargetSDK = PresentAssumeLatestStable()
	}
	if cfgMinSDK > 0 {
		android.MinSDK = KnownVersion(strconv.Itoa(cfgMinSDK))
	}

	if version := firstMatch(javaToolchainRe, content); version != "" {
		jvm.ToolchainLanguageVersion = KnownVersion(normalizeVersion(version))
	}
	if version := firstMatch(sourceCompatibilityRe, content); version != "" {
		jvm.SourceCompatibility = KnownVersion(normalizeVersion(version))
	}
	if version := firstMatch(targetCompatibilityRe, content); version != "" {
		jvm.TargetCompatibility = KnownVersion(normalizeVersion(version))
	}
	if version := firstMatch(kotlinJvmTargetRe, content); version != "" {
		jvm.KotlinJvmTarget = KnownVersion(normalizeVersion(version))
	} else if kotlin.Compiler.Presence == PresencePresent {
		jvm.KotlinJvmTarget = PresentAssumeLatestStable()
	}

	return kotlin, ksp, android, jvm
}

func firstMatch(re *regexp.Regexp, content string) string {
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func normalizeVersion(version string) string {
	return strings.ReplaceAll(strings.TrimSpace(version), "_", ".")
}

func mergeKotlinProfile(current, next KotlinProfile) KotlinProfile {
	current.Compiler = mergeVersionedPresence(current.Compiler, next.Compiler)
	current.LanguageVersion = mergeVersionedPresence(current.LanguageVersion, next.LanguageVersion)
	current.ApiVersion = mergeVersionedPresence(current.ApiVersion, next.ApiVersion)
	if current.K2 == PresenceUnknown {
		current.K2 = next.K2
	} else if next.K2 == PresencePresent {
		current.K2 = PresencePresent
	}
	return current
}

func mergeKSPProfile(current, next KSPProfile) KSPProfile {
	current.Tool = mergeVersionedPresence(current.Tool, next.Tool)
	current.Processors = append(current.Processors, next.Processors...)
	return current
}

func mergeAndroidProfile(current, next AndroidProfile) AndroidProfile {
	current.AGP = mergeVersionedPresence(current.AGP, next.AGP)
	current.CompileSDK = mergeVersionedPresence(current.CompileSDK, next.CompileSDK)
	current.TargetSDK = mergeVersionedPresence(current.TargetSDK, next.TargetSDK)
	current.MinSDK = mergeVersionedPresence(current.MinSDK, next.MinSDK)
	return current
}

func mergeJVMProfile(current, next JVMProfile) JVMProfile {
	current.ToolchainLanguageVersion = mergeVersionedPresence(current.ToolchainLanguageVersion, next.ToolchainLanguageVersion)
	current.SourceCompatibility = mergeVersionedPresence(current.SourceCompatibility, next.SourceCompatibility)
	current.TargetCompatibility = mergeVersionedPresence(current.TargetCompatibility, next.TargetCompatibility)
	current.KotlinJvmTarget = mergeVersionedPresence(current.KotlinJvmTarget, next.KotlinJvmTarget)
	return current
}
