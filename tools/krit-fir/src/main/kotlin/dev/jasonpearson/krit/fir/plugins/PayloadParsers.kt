package dev.jasonpearson.krit.fir.plugins

/**
 * Minimal JSON helpers for slicing nested objects + scalars out of an
 * `analyzeFile` request body. Mirrors the krit-types' parser surface
 * exactly so wire-format drift between the two backends is impossible
 * — same expression types come out the same Go-side struct on either
 * side.
 *
 * Implemented by hand because adding a JSON dependency to krit-fir's
 * shadow jar would bloat the wire-format JAR by megabytes for a use
 * case (analyzeFile request decoding) that hits the parser on the
 * order of once per IDE keystroke.
 */
internal object PayloadParsers {

    /**
     * Return the JSON object body for `"<key>": {...}` — the inner
     * `{...}` slice including braces. Brace-balanced so nested
     * objects don't terminate the slice early. Returns null when the
     * key is absent or its value is not an object.
     */
    fun extractObjectBlock(json: String, key: String): String? {
        val keyIdx = findKeyStart(json, key) ?: return null
        var i = json.indexOf(':', keyIdx)
        if (i < 0) return null
        i = skipWhitespace(json, i + 1)
        if (i >= json.length || json[i] != '{') return null
        val start = i
        var depth = 0
        var inString = false
        var escape = false
        while (i < json.length) {
            val c = json[i]
            when {
                escape -> escape = false
                inString && c == '\\' -> escape = true
                inString && c == '"' -> inString = false
                !inString && c == '"' -> inString = true
                !inString && c == '{' -> depth++
                !inString && c == '}' -> {
                    depth--
                    if (depth == 0) return json.substring(start, i + 1)
                }
            }
            i++
        }
        return null
    }

    /**
     * Reads a string-valued field out of a JSON object. Handles
     * `\\"`, `\\\\`, `\\n`, `\\r`, `\\t`, and `\\uXXXX` — the same
     * escape set the wire encoder produces.
     */
    fun extractString(json: String, key: String): String? {
        val keyIdx = findKeyStart(json, key) ?: return null
        var i = json.indexOf(':', keyIdx)
        if (i < 0) return null
        i = skipWhitespace(json, i + 1)
        if (i >= json.length || json[i] != '"') return null
        i++
        val sb = StringBuilder()
        while (i < json.length) {
            val c = json[i]
            when {
                c == '"' -> return sb.toString()
                c == '\\' && i + 1 < json.length -> {
                    when (val esc = json[i + 1]) {
                        '"', '\\', '/' -> sb.append(esc)
                        'n' -> sb.append('\n')
                        'r' -> sb.append('\r')
                        't' -> sb.append('\t')
                        'b' -> sb.append('\b')
                        'f' -> sb.append('\u000C')
                        'u' -> {
                            if (i + 5 >= json.length) return null
                            val code = json.substring(i + 2, i + 6).toIntOrNull(16) ?: return null
                            sb.append(code.toChar())
                            i += 4
                        }
                        else -> sb.append(esc)
                    }
                    i += 2
                    continue
                }
                else -> sb.append(c)
            }
            i++
        }
        return null
    }

    /** Reads an integer field. Returns null on absent or non-integer. */
    fun extractLong(json: String, key: String): Long? {
        val keyIdx = findKeyStart(json, key) ?: return null
        var i = json.indexOf(':', keyIdx)
        if (i < 0) return null
        i = skipWhitespace(json, i + 1)
        val start = i
        if (i < json.length && (json[i] == '-' || json[i] == '+')) i++
        while (i < json.length && json[i].isDigit()) i++
        if (i == start) return null
        return json.substring(start, i).toLongOrNull()
    }

    /**
     * Read a `["a","b","c"]` array of strings. Returns null when the
     * key is absent; returns empty list when the array is present but
     * empty. Inner strings honour the same escape set as
     * [`extractString`].
     */
    fun extractStringArray(json: String, key: String): List<String>? {
        val keyIdx = findKeyStart(json, key) ?: return null
        var i = json.indexOf(':', keyIdx)
        if (i < 0) return null
        i = skipWhitespace(json, i + 1)
        if (i >= json.length || json[i] != '[') return null
        i++
        val out = mutableListOf<String>()
        while (i < json.length) {
            i = skipWhitespace(json, i)
            if (i >= json.length) return null
            if (json[i] == ']') return out
            if (json[i] != '"') return null
            // Slice from the open quote forward and let extractString
            // consume the literal. Wrapping it in a 1-element object
            // (`{"v":"..."}`) lets us reuse the same helper without a
            // dedicated array-element parser.
            val rest = "{\"v\":" + json.substring(i)
            val value = extractString(rest, "v") ?: return null
            out.add(value)
            // Advance past the literal we just consumed. Walk through
            // the original string honouring escapes so we land on the
            // closing quote precisely.
            i++ // step over opening quote
            var escape = false
            while (i < json.length) {
                val c = json[i]
                if (escape) {
                    escape = false
                } else if (c == '\\') {
                    escape = true
                } else if (c == '"') {
                    i++
                    break
                }
                i++
            }
            i = skipWhitespace(json, i)
            if (i < json.length && json[i] == ',') i++
        }
        return null
    }

    /** Find the index of the opening `"` of `"<key>"` in [json]. */
    private fun findKeyStart(json: String, key: String): Int? {
        val needle = "\"" + key + "\""
        var from = 0
        while (true) {
            val idx = json.indexOf(needle, from)
            if (idx < 0) return null
            // Reject substring matches inside other key names (e.g.
            // `"sourceDirs"` matching `key="source"`).
            val before = idx - 1
            if (before >= 0) {
                val b = json[before]
                if (b != '{' && b != ',' && !b.isWhitespace()) {
                    from = idx + needle.length
                    continue
                }
            }
            return idx
        }
    }

    private fun skipWhitespace(json: String, start: Int): Int {
        var i = start
        while (i < json.length && json[i].isWhitespace()) i++
        return i
    }
}

/**
 * Convenience that pulls all five project-scope payload objects out
 * of an `analyzeFile` request body in one pass. Each field is null
 * when the corresponding top-level key is absent.
 */
data class ProjectPayloads(
    val gradle: GradleProfilePayload?,
    val manifest: ManifestProfilePayload?,
    val resources: ResourcesProfilePayload?,
    val moduleIndex: ModulesProfilePayload?,
    val crossFile: CrossFileProfilePayload?,
) {
    companion object {
        val EMPTY: ProjectPayloads = ProjectPayloads(null, null, null, null, null)

        fun parse(json: String): ProjectPayloads = ProjectPayloads(
            gradle = parseGradle(json),
            manifest = parseManifest(json),
            resources = parseResources(json),
            moduleIndex = parseModuleIndex(json),
            crossFile = parseCrossFile(json),
        )

        private fun parseGradle(json: String): GradleProfilePayload? {
            val outer = PayloadParsers.extractObjectBlock(json, "gradle") ?: return null
            return GradleProfilePayload(
                minSdk = PayloadParsers.extractLong(outer, "minSdk")?.toInt()?.takeIf { it >= 0 },
                targetSdk = PayloadParsers.extractLong(outer, "targetSdk")?.toInt()?.takeIf { it >= 0 },
                compileSdk = PayloadParsers.extractLong(outer, "compileSdk")?.toInt()?.takeIf { it >= 0 },
                kotlinVersion = PayloadParsers.extractString(outer, "kotlinVersion"),
                javaTargetVersion = PayloadParsers.extractString(outer, "javaTargetVersion"),
                agpVersion = PayloadParsers.extractString(outer, "agpVersion"),
                deps = PayloadParsers.extractStringArray(outer, "deps").orEmpty(),
            )
        }

        private fun parseManifest(json: String): ManifestProfilePayload? {
            val outer = PayloadParsers.extractObjectBlock(json, "manifest") ?: return null
            return ManifestProfilePayload(
                packageName = PayloadParsers.extractString(outer, "package"),
                minSdk = PayloadParsers.extractLong(outer, "minSdk")?.toInt()?.takeIf { it >= 0 },
                targetSdk = PayloadParsers.extractLong(outer, "targetSdk")?.toInt()?.takeIf { it >= 0 },
                permissions = PayloadParsers.extractStringArray(outer, "permissions").orEmpty(),
                activities = PayloadParsers.extractStringArray(outer, "activities").orEmpty(),
                exportedActivities = PayloadParsers.extractStringArray(outer, "exportedActivities").orEmpty(),
                services = PayloadParsers.extractStringArray(outer, "services").orEmpty(),
                exportedServices = PayloadParsers.extractStringArray(outer, "exportedServices").orEmpty(),
                receivers = PayloadParsers.extractStringArray(outer, "receivers").orEmpty(),
                exportedReceivers = PayloadParsers.extractStringArray(outer, "exportedReceivers").orEmpty(),
            )
        }

        private fun parseResources(json: String): ResourcesProfilePayload? {
            val outer = PayloadParsers.extractObjectBlock(json, "resources") ?: return null
            return ResourcesProfilePayload(
                strings = PayloadParsers.extractStringArray(outer, "strings").orEmpty(),
                drawables = PayloadParsers.extractStringArray(outer, "drawables").orEmpty(),
                layouts = PayloadParsers.extractStringArray(outer, "layouts").orEmpty(),
                colors = PayloadParsers.extractStringArray(outer, "colors").orEmpty(),
                dimensions = PayloadParsers.extractStringArray(outer, "dimensions").orEmpty(),
                ids = PayloadParsers.extractStringArray(outer, "ids").orEmpty(),
            )
        }

        private fun parseModuleIndex(json: String): ModulesProfilePayload? {
            val outer = PayloadParsers.extractObjectBlock(json, "moduleIndex") ?: return null
            return ModulesProfilePayload(
                modules = PayloadParsers.extractStringArray(outer, "modules").orEmpty(),
            )
        }

        private fun parseCrossFile(json: String): CrossFileProfilePayload? {
            val outer = PayloadParsers.extractObjectBlock(json, "crossFile") ?: return null
            return CrossFileProfilePayload(
                declarations = PayloadParsers.extractStringArray(outer, "declarations").orEmpty(),
                nonCommentRefsByName = PayloadParsers.extractStringArray(outer, "nonCommentRefsByName").orEmpty(),
            )
        }
    }
}
