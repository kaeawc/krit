package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.CrossFileContext
import dev.jasonpearson.krit.api.CrossFileDeclaration
import dev.jasonpearson.krit.api.GradleContext
import dev.jasonpearson.krit.api.ManifestContext
import dev.jasonpearson.krit.api.ModuleIndexContext
import dev.jasonpearson.krit.api.ResourcesContext

/**
 * Wire-format project facts handed to plugin rules via [`RuleContext`].
 * Mirrors the payload shape krit-types serializes in its `analyzeFile`
 * request so a rule jar that worked against the KAA backend ships
 * identical fact access on the FIR backend after rebuild.
 *
 * Each payload class carries the flat, easy-to-parse wire shape
 * (`List<String>` plus a handful of scalars); the matching
 * `Payload*Context` wrappers build the per-component lookup maps
 * lazily so rules that only query one direction don't pay for the
 * other.
 */

data class GradleProfilePayload(
    val minSdk: Int?,
    val targetSdk: Int?,
    val compileSdk: Int?,
    val kotlinVersion: String?,
    val javaTargetVersion: String?,
    val agpVersion: String?,
    // `group:name:version` triples, krit-types-compatible.
    val deps: List<String>,
)

data class ManifestProfilePayload(
    val packageName: String?,
    val minSdk: Int?,
    val targetSdk: Int?,
    val permissions: List<String>,
    val activities: List<String>,
    val exportedActivities: List<String>,
    val services: List<String>,
    val exportedServices: List<String>,
    val receivers: List<String>,
    val exportedReceivers: List<String>,
)

data class ResourcesProfilePayload(
    // Strings / colors / dimensions are flat `name=value` lists so the
    // wire format reuses the same `String[]` extractor — the wrapper
    // parses them back into maps on first query.
    val strings: List<String>,
    val drawables: List<String>,
    val layouts: List<String>,
    val colors: List<String>,
    val dimensions: List<String>,
    val ids: List<String>,
)

data class ModulesProfilePayload(
    // Each module is `path|directory|dependsOn,...|sourceRoots,...`.
    val modules: List<String>,
)

data class CrossFileProfilePayload(
    // Declarations are `fqn|kind|file|line|visibility`.
    val declarations: List<String>,
    // Pre-grouped non-comment references as `name|file1,file2,...`.
    val nonCommentRefsByName: List<String>,
)

internal class PayloadGradleContext(payload: GradleProfilePayload) : GradleContext {
    override val minSdk: Int? = payload.minSdk
    override val targetSdk: Int? = payload.targetSdk
    override val compileSdk: Int? = payload.compileSdk
    override val kotlinVersion: String? = payload.kotlinVersion
    override val javaTargetVersion: String? = payload.javaTargetVersion
    override val agpVersion: String? = payload.agpVersion

    // group:name -> version (last-write-wins on duplicate coords; the
    // Go-side caller already de-dupes, but Kotlin's map semantics
    // protect against a malformed payload).
    private val versionByCoord: Map<String, String> by lazy {
        val out = HashMap<String, String>(payload.deps.size)
        for (entry in payload.deps) {
            val firstColon = entry.indexOf(':')
            if (firstColon <= 0) continue
            val secondColon = entry.indexOf(':', firstColon + 1)
            if (secondColon <= firstColon + 1) continue
            val coord = entry.substring(0, secondColon)
            val version = entry.substring(secondColon + 1)
            if (version.isNotEmpty()) {
                out[coord] = version
            }
        }
        out
    }

    override fun hasDependency(group: String, name: String): Boolean =
        versionByCoord.containsKey("$group:$name")

    override fun dependencyVersion(group: String, name: String): String? =
        versionByCoord["$group:$name"]
}

internal class PayloadManifestContext(payload: ManifestProfilePayload) : ManifestContext {
    override val packageName: String? = payload.packageName
    override val minSdk: Int? = payload.minSdk
    override val targetSdk: Int? = payload.targetSdk

    private val permissionSet: Set<String> by lazy { payload.permissions.toHashSet() }
    private val activitySet: Set<String> by lazy { payload.activities.toHashSet() }
    private val exportedActivitySet: Set<String> by lazy { payload.exportedActivities.toHashSet() }
    private val serviceSet: Set<String> by lazy { payload.services.toHashSet() }
    private val exportedServiceSet: Set<String> by lazy { payload.exportedServices.toHashSet() }
    private val receiverSet: Set<String> by lazy { payload.receivers.toHashSet() }
    private val exportedReceiverSet: Set<String> by lazy { payload.exportedReceivers.toHashSet() }

    override fun hasPermission(name: String): Boolean = name in permissionSet
    override fun hasActivity(name: String): Boolean = name in activitySet
    override fun isActivityExported(name: String): Boolean = name in exportedActivitySet
    override fun hasService(name: String): Boolean = name in serviceSet
    override fun isServiceExported(name: String): Boolean = name in exportedServiceSet
    override fun hasReceiver(name: String): Boolean = name in receiverSet
    override fun isReceiverExported(name: String): Boolean = name in exportedReceiverSet
}

internal class PayloadResourcesContext(payload: ResourcesProfilePayload) : ResourcesContext {
    private val stringMap: Map<String, String> by lazy { parseNameValueList(payload.strings) }
    private val colorMap: Map<String, String> by lazy { parseNameValueList(payload.colors) }
    private val dimensionMap: Map<String, String> by lazy { parseNameValueList(payload.dimensions) }
    private val drawableSet: Set<String> by lazy { payload.drawables.toHashSet() }
    private val layoutSet: Set<String> by lazy { payload.layouts.toHashSet() }
    private val idSet: Set<String> by lazy { payload.ids.toHashSet() }

    override fun stringValue(name: String): String? = stringMap[name]
    override fun hasString(name: String): Boolean = stringMap.containsKey(name)
    override fun hasDrawable(name: String): Boolean = name in drawableSet
    override fun hasLayout(name: String): Boolean = name in layoutSet
    override fun colorValue(name: String): String? = colorMap[name]
    override fun hasColor(name: String): Boolean = colorMap.containsKey(name)
    override fun dimensionValue(name: String): String? = dimensionMap[name]
    override fun hasDimension(name: String): Boolean = dimensionMap.containsKey(name)
    override fun hasId(name: String): Boolean = name in idSet

    private fun parseNameValueList(entries: List<String>): Map<String, String> {
        val out = HashMap<String, String>(entries.size)
        for (entry in entries) {
            val eq = entry.indexOf('=')
            // eq <= 0 catches both "no = found" (-1) and "leading =" (0,
            // which would make the name empty). Either is malformed.
            if (eq <= 0) continue
            out[entry.substring(0, eq)] = entry.substring(eq + 1)
        }
        return out
    }
}

internal class PayloadModuleIndexContext(payload: ModulesProfilePayload) : ModuleIndexContext {
    private data class Entry(
        val directory: String,
        val dependsOn: List<String>,
        val sourceRoots: List<String>,
    )

    private val byPath: Map<String, Entry> by lazy {
        val out = LinkedHashMap<String, Entry>(payload.modules.size)
        for (line in payload.modules) {
            val parts = line.split('|')
            if (parts.size < 4 || parts[0].isEmpty()) continue
            val deps = parts[2].takeIf { it.isNotEmpty() }?.split(',') ?: emptyList()
            val roots = parts[3].takeIf { it.isNotEmpty() }?.split(',') ?: emptyList()
            out[parts[0]] = Entry(parts[1], deps, roots)
        }
        out
    }

    override val modulePaths: List<String> get() = byPath.keys.toList()
    override fun directoryOf(modulePath: String): String? = byPath[modulePath]?.directory
    override fun dependenciesOf(modulePath: String): List<String> =
        byPath[modulePath]?.dependsOn.orEmpty()
    override fun sourceRootsOf(modulePath: String): List<String> =
        byPath[modulePath]?.sourceRoots.orEmpty()
}

internal class PayloadCrossFileContext(payload: CrossFileProfilePayload) : CrossFileContext {
    private val byFqn: Map<String, CrossFileDeclaration> by lazy {
        val out = HashMap<String, CrossFileDeclaration>(payload.declarations.size)
        for (entry in payload.declarations) {
            val parts = entry.split('|')
            if (parts.size < 4 || parts[0].isEmpty()) continue
            val line = parts[3].toIntOrNull() ?: continue
            val visibility = parts.getOrNull(4)?.takeIf { it.isNotEmpty() }
            out[parts[0]] = CrossFileDeclaration(parts[0], parts[1], parts[2], line, visibility)
        }
        out
    }

    private val refFilesByName: Map<String, List<String>> by lazy {
        val out = HashMap<String, List<String>>(payload.nonCommentRefsByName.size)
        for (entry in payload.nonCommentRefsByName) {
            val sep = entry.indexOf('|')
            if (sep <= 0) continue
            val name = entry.substring(0, sep)
            val files = entry.substring(sep + 1)
                .split(',')
                .filter { it.isNotEmpty() }
            if (files.isNotEmpty()) {
                out[name] = files
            }
        }
        out
    }

    override fun declarationByFqn(fqn: String): CrossFileDeclaration? = byFqn[fqn]
    override fun referenceFiles(name: String): List<String> = refFilesByName[name].orEmpty()
    override fun isReferenced(name: String): Boolean = refFilesByName.containsKey(name)
}
