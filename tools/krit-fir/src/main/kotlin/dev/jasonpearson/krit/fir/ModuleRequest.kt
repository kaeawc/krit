package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.FileRef
import dev.jasonpearson.krit.fir.runner.ModuleFragment
import dev.jasonpearson.krit.fir.runner.ModuleSpec

/** Structured parsing keeps module-local classpaths and IDs out of legacy nest-blind extraction. */
internal fun parseModuleRequest(json: String, oneShot: Boolean = false): CheckRequest? {
    val root = ConfigJsonReader(json, "modules request").value() as? Map<*, *> ?: return null
    val payload = root["params"] as? Map<*, *> ?: root
    val values = payload["modules"] ?: return null
    require(values is List<*>) { "modules must be an array" }
    fun Map<*, *>.strings(key: String): List<String> {
        val value = this[key] ?: return emptyList()
        require(value is List<*> && value.all { it is String }) { "$key must be a string array" }
        return value.filterIsInstance<String>()
    }
    fun Map<*, *>.string(key: String, default: String? = null): String =
        (this[key] as? String ?: default ?: error("Missing $key")).also { require(it.isNotEmpty()) { "Empty $key" } }
    val modules = values.map { value ->
        val m = value as? Map<*, *> ?: error("module must be an object")
        val fragments = (m["fragments"] as? List<*>).orEmpty().map { item ->
            val f = item as? Map<*, *> ?: error("fragment must be an object")
            ModuleFragment(f.string("name"), f.strings("sourceRoots"), f.strings("refines"))
        }
        ModuleSpec(m.string("id"), m.string("platform", "jvm"), m.string("kind", "main"),
            m.strings("sourceRoots"), m.strings("generatedSourceRoots"), m.strings("classpath"),
            m.strings("dependsOn"), m.strings("friends"), m.strings("compilerArgs"),
            m.string("jvmTarget", "1.8"), fragments)
    }
    @Suppress("UNCHECKED_CAST")
    val configs = (payload["ruleConfigs"] as? Map<String, Map<String, Any?>>).orEmpty()
    @Suppress("UNCHECKED_CAST")
    val scanPaths = (payload["scanPaths"] as? Map<String, String>).orEmpty()
    val files = if ("checkFiles" in payload) payload.strings("checkFiles").map { FileRef(it) } else {
        (payload["files"] as? List<*>).orEmpty().map {
            when (it) {
                is String -> FileRef(it)
                is Map<*, *> -> FileRef(it.string("path"), it["contentHash"] as? String ?: "")
                else -> error("Invalid file reference")
            }
        }
    }
    return CheckRequest(
        id = (root["id"] as? Number)?.toLong() ?: if (oneShot) 0 else error("Missing id"),
        command = root["command"] as? String ?: root["method"] as? String ?: if (oneShot) "analyzeModules" else error("Missing command"),
        files = files, sourceDirs = payload.strings("sourceDirs"), classpath = payload.strings("classpath"),
        rules = payload.strings("rules"), ruleConfigs = configs, testFiles = payload.strings("testFiles").toSet(),
        scanPaths = scanPaths, sdkLevels = sdkLevelsOf(payload["sdkLevels"]), modules = modules,
    )
}
