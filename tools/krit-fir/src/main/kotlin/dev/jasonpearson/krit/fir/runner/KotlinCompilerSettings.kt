package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.arguments.parseCommandLineArguments
import org.jetbrains.kotlin.config.LanguageVersion

/**
 * The project's effective Kotlin compiler settings, as krit's Gradle model
 * exporter and Go pipeline resolved them. Without any, the embedded compiler's
 * own defaults apply (the documented behavior for a missing or incomplete
 * Gradle model).
 *
 * Only settings that change how source is analyzed are applied: the language
 * and API level, opt-in markers, progressive mode and an explicit allowlist of
 * `-X` / `-XXLanguage:` flags. Everything else is reported in [warnings] and
 * left out, never applied silently and never dropped silently. In particular
 * the arguments krit owns (output, classpath, plugin, JDK, module name, friend
 * paths and HMPP fragments) can never be set from here.
 *
 * The Go side filters with the same rules (`internal/gradlemodel`
 * `kotlin_settings.go`); this is the helper's own copy so a request from any
 * other client is held to the same allowlist. A Go test keeps the flag lists
 * equal: every line below tagged `allowlist:` is one allowlisted flag.
 */
data class KotlinCompilerSettings(
    val languageVersion: String? = null,
    val apiVersion: String? = null,
    val optIns: List<String> = emptyList(),
    val progressive: Boolean = false,
    val flags: List<String> = emptyList(),
    /** Arguments or values that were not applied, each with the reason. */
    val warnings: List<String> = emptyList(),
) {
    /** True when nothing would reach the compiler. */
    val isEmpty: Boolean
        get() = languageVersion == null && apiVersion == null && optIns.isEmpty() && !progressive && flags.isEmpty()

    /** Canonical form of the applied settings, for cache keys and session comparison. */
    val identity: String
        get() = if (isEmpty) "" else listOf(
            "lang=$languageVersion", "api=$apiVersion", "progressive=$progressive",
            "optin=" + optIns.sorted().joinToString("\u001f"), "flags=" + flags.joinToString("\u001f"),
        ).joinToString("|")

    /** The applied settings in the equals-form argument list `parse` reads back. */
    val arguments: List<String>
        get() = buildList {
            languageVersion?.let { add("-language-version=$it") }
            apiVersion?.let { add("-api-version=$it") }
            optIns.sorted().forEach { add("-opt-in=$it") }
            if (progressive) add("-progressive")
            addAll(flags)
        }

    /** Applies the settings to [args]; krit-owned fields are never touched. */
    fun applyTo(args: K2JVMCompilerArguments) {
        languageVersion?.let { args.languageVersion = it }
        apiVersion?.let { args.apiVersion = it }
        if (optIns.isNotEmpty()) args.optIn = ((args.optIn?.toList() ?: emptyList()) + optIns).distinct().toTypedArray()
        if (progressive) args.progressiveMode = true
        // Only allowlisted flags reach the parser, so it cannot set a krit-owned field.
        if (flags.isNotEmpty()) parseCommandLineArguments(flags, args)
    }

    companion object {
        /** Flag name to whether it takes a `=value`. */
        internal val ALLOWLIST: Map<String, Boolean> = mapOf(
            "-Xannotation-default-target" to true, // allowlist:
            "-Xannotation-target-all" to false, // allowlist:
            "-Xcontext-parameters" to false, // allowlist:
            "-Xcontext-receivers" to false, // allowlist:
            "-Xcontext-sensitive-resolution" to false, // allowlist:
            "-Xconsistent-data-class-copy-visibility" to false, // allowlist:
            "-Xdata-flow-based-exhaustiveness" to false, // allowlist:
            "-Xenhance-type-parameter-types-to-def-not-null" to false, // allowlist:
            "-Xexplicit-backing-fields" to false, // allowlist:
            "-Xexpect-actual-classes" to false, // allowlist:
            "-Xjsr305" to true, // allowlist:
            "-Xjvm-default" to true, // allowlist:
            "-Xmulti-dollar-interpolation" to false, // allowlist:
            "-Xnested-type-aliases" to false, // allowlist:
            "-Xnon-local-break-continue" to false, // allowlist:
            "-Xnullability-annotations" to true, // allowlist:
            "-Xreturn-value-checker" to true, // allowlist:
            "-Xtype-enhancement-improvements-strict-mode" to false, // allowlist:
            "-Xunrestricted-builder-inference" to false, // allowlist:
            "-Xwhen-guards" to false, // allowlist:
        )

        /** Arguments krit owns; the valued ones take the next token in split form. */
        private val PROTECTED = setOf(
            "-d", "-classpath", "-cp", "-jvm-target", "-module-name", "-P", "-jdk-home", "-Xfriend-paths", "-Xplugin",
            "-Xjdk-release", "-Xadd-modules", "-Xbuild-file", "-Xmodule-path", "-Xmulti-platform", "-Xcommon-sources",
            "-Xexpect-actual-linker", "-no-stdlib", "-no-reflect", "-Xno-stdlib", "-Xno-reflect", "-Xcompiler-plugin",
            "-script-templates",
        )
        private val PROTECTED_VALUED = setOf("-d", "-classpath", "-cp", "-jvm-target", "-module-name", "-P", "-jdk-home", "-script-templates")

        private val VERSION = Regex("^[0-9]+\\.[0-9]+$")
        private val OPT_IN = Regex("^[A-Za-z_][A-Za-z0-9_.]*$")
        private val FLAG_VALUE = Regex("^[A-Za-z0-9_.,:+@\\-]+$")
        private val LANGUAGE_TOGGLE = Regex("^-XXLanguage:[+-][A-Za-z][A-Za-z0-9_]*$")

        internal fun isProtected(arg: String): Boolean {
            val name = arg.substringBefore('=')
            return name in PROTECTED || name.startsWith("-Xfragment") || name.startsWith("-Xplugin")
        }

        /** Classifies [args]: allowlisted settings are applied, everything else becomes a warning. */
        fun parse(args: List<String>): KotlinCompilerSettings {
            var language: String? = null
            var api: String? = null
            val optIns = mutableListOf<String>()
            var progressive = false
            val flags = mutableListOf<String>()
            val flagKeys = mutableListOf<String>()
            val warnings = mutableListOf<String>()
            fun reject(arg: String, reason: String) { warnings += "ignoring compiler argument $arg ($reason)" }
            var i = 0
            while (i < args.size) {
                val arg = args[i++].trim()
                if (arg.isEmpty()) continue
                val name = arg.substringBefore('=')
                val hasValue = '=' in arg
                // Operand of a valued option in equals or split form.
                fun operand(): String? = if (hasValue) arg.substringAfter('=') else args.getOrNull(i)?.trim()?.also { i++ }
                when {
                    name == "-language-version" || name == "-api-version" -> {
                        val v = operand()
                        if (v == null || !VERSION.matches(v)) reject(arg, "unsupported version value")
                        else if (name == "-language-version") language = v else api = v
                    }
                    name == "-opt-in" || name == "-Xopt-in" -> {
                        val v = operand()
                        if (v == null || !OPT_IN.matches(v)) reject(arg, "unsupported opt-in marker") else optIns += v
                    }
                    name == "-progressive" -> when {
                        !hasValue || arg.substringAfter('=') == "true" -> progressive = true
                        arg.substringAfter('=') == "false" -> progressive = false
                        else -> reject(arg, "flag takes true or false")
                    }
                    isProtected(arg) -> {
                        if (name in PROTECTED_VALUED && !hasValue && i < args.size) reject("$arg ${args[i++]}", "managed by krit")
                        else reject(arg, "managed by krit")
                    }
                    LANGUAGE_TOGGLE.matches(arg) -> {
                        replaceFlag(flags, flagKeys, "-XXLanguage:" + arg.substring("-XXLanguage:".length + 1), arg)
                    }
                    else -> {
                        val takesValue = ALLOWLIST[name]
                        when {
                            takesValue == null -> reject(arg, "not an analysis-semantic setting krit applies")
                            !takesValue && hasValue -> reject(arg, "flag takes no value")
                            takesValue && (!hasValue || !FLAG_VALUE.matches(arg.substringAfter('='))) ->
                                reject(arg, "flag needs a simple value")
                            else -> replaceFlag(flags, flagKeys, name, arg)
                        }
                    }
                }
            }
            // Versions the embedded compiler cannot honor are reported, not guessed at.
            val knownLanguage = language?.let { LanguageVersion.fromVersionString(it) }
            if (language != null && (knownLanguage == null || knownLanguage.isUnsupported)) {
                warnings += "languageVersion $language is not supported by the embedded compiler; its default applies"
                language = null
            }
            // An API level mirrors a language level; the compiler rejects an unsupported
            // one outright ("API version 1.9 is no longer supported"), so it is dropped too.
            val knownApi = api?.let { LanguageVersion.fromVersionString(it) }
            if (api != null && (knownApi == null || knownApi.isUnsupported)) {
                warnings += "apiVersion $api is not supported by the embedded compiler; its default applies"
                api = null
            }
            // The compiler rejects an API level above the language level; the API level then
            // follows the language level, as Go decides too.
            val effectiveLanguage = language?.let { LanguageVersion.fromVersionString(it) }
            val effectiveApi = api?.let { LanguageVersion.fromVersionString(it) }
            if (effectiveLanguage != null && effectiveApi != null && effectiveApi > effectiveLanguage) {
                warnings += "apiVersion $api is greater than languageVersion $language; not applied"
                api = null
            }
            return KotlinCompilerSettings(language, api, optIns.distinct().sorted(), progressive, flags.toList(), warnings)
        }

        /** The last occurrence of a flag (or -XXLanguage feature) wins and keeps its position. */
        private fun replaceFlag(flags: MutableList<String>, keys: MutableList<String>, key: String, arg: String) {
            val at = keys.indexOf(key)
            if (at >= 0) { flags.removeAt(at); keys.removeAt(at) }
            flags += arg
            keys += key
        }
    }
}
