package dev.jasonpearson.krit.fir.oracle

/**
 * Daemon-response builders for the oracle-style RPC methods (`analyze`,
 * `analyzeAll`, `analyzeFiles`, `analyzeWithDeps`). The wire shape
 * mirrors `buildDaemonResponse` in krit-types' `Main.kt` so a single
 * Go-side client can talk to either backend.
 *
 * Per-file payloads come in as a structured [AnalyzeResult]; the
 * serialization preserves the krit-types JSON shape regardless of how
 * populated the result is. Callers handing [AnalyzeResult.EMPTY] get an
 * envelope shell with empty `files` and `dependencies` maps; callers
 * handing a populated result get the same envelope with the per-file
 * payloads filled in.
 */
object OracleResponse {

    /**
     * Build the krit-types-compatible envelope for an analyze response.
     * Errors carried on [result] are nested inside the `result` object
     * to match krit-types' legacy `buildDaemonResponse` shape; the flat
     * `cacheDeps`-sibling envelope used by `analyzeWithDeps` is
     * separate and lands with the cacheDeps projection in a later PR.
     */
    fun buildAnalyze(id: Long, result: AnalyzeResult = AnalyzeResult.EMPTY): String {
        val sb = StringBuilder()
        sb.append("""{"id":""")
        sb.append(id)
        sb.append(""","result":{""")
        sb.append(""""version":1,""")
        sb.append(""""kotlinVersion":""")
        sb.append(jsonString(KotlinVersion.CURRENT.toString()))
        sb.append(""","files":""")
        appendFiles(sb, result.files)
        sb.append(""","dependencies":""")
        appendDependencies(sb, result.dependencies)
        if (result.errors.isNotEmpty()) {
            sb.append(""","errors":""")
            appendErrors(sb, result.errors)
        }
        sb.append("}}")
        return sb.toString()
    }

    private fun appendFiles(sb: StringBuilder, files: Map<String, FilePayload>) {
        if (files.isEmpty()) {
            sb.append("{}")
            return
        }
        sb.append("{")
        files.entries.forEachIndexed { i, (path, payload) ->
            if (i > 0) sb.append(",")
            sb.append(jsonString(path))
            sb.append(":")
            appendFilePayload(sb, payload)
        }
        sb.append("}")
    }

    private fun appendFilePayload(sb: StringBuilder, payload: FilePayload) {
        sb.append("{")
        sb.append(""""package":""")
        sb.append(jsonString(payload.packageName))
        sb.append(""","declarations":""")
        appendClassList(sb, payload.declarations)
        sb.append(""","expressions":""")
        appendExpressions(sb, payload.expressions)
        if (payload.diagnostics.isNotEmpty()) {
            sb.append(""","diagnostics":""")
            appendDiagnostics(sb, payload.diagnostics)
        }
        sb.append("}")
    }

    private fun appendDependencies(sb: StringBuilder, deps: Map<String, ClassPayload>) {
        if (deps.isEmpty()) {
            sb.append("{}")
            return
        }
        sb.append("{")
        deps.entries.forEachIndexed { i, (fqn, cls) ->
            if (i > 0) sb.append(",")
            sb.append(jsonString(fqn))
            sb.append(":")
            appendClass(sb, cls)
        }
        sb.append("}")
    }

    private fun appendClassList(sb: StringBuilder, classes: List<ClassPayload>) {
        sb.append("[")
        classes.forEachIndexed { i, cls ->
            if (i > 0) sb.append(",")
            appendClass(sb, cls)
        }
        sb.append("]")
    }

    private fun appendClass(sb: StringBuilder, cls: ClassPayload) {
        sb.append("{")
        sb.append(""""fqn":""")
        sb.append(jsonString(cls.fqn))
        sb.append(""","kind":""")
        sb.append(jsonString(cls.kind))
        sb.append(""","supertypes":""")
        appendStringList(sb, cls.supertypes)
        sb.append(""","visibility":""")
        sb.append(jsonString(cls.visibility))
        if (cls.isSealed) sb.append(""","isSealed":true""")
        if (cls.isData) sb.append(""","isData":true""")
        if (cls.isOpen) sb.append(""","isOpen":true""")
        if (cls.isAbstract) sb.append(""","isAbstract":true""")
        if (cls.typeParameters.isNotEmpty()) {
            sb.append(""","typeParameters":""")
            appendStringList(sb, cls.typeParameters)
        }
        if (cls.members.isNotEmpty()) {
            sb.append(""","members":""")
            appendMembers(sb, cls.members)
        }
        if (cls.annotations.isNotEmpty()) {
            sb.append(""","annotations":""")
            appendStringList(sb, cls.annotations)
        }
        cls.jarPath?.let {
            sb.append(""","jarPath":""")
            sb.append(jsonString(it))
        }
        sb.append("}")
    }

    private fun appendMembers(sb: StringBuilder, members: List<MemberPayload>) {
        sb.append("[")
        members.forEachIndexed { i, m ->
            if (i > 0) sb.append(",")
            sb.append("{")
            sb.append(""""name":""")
            sb.append(jsonString(m.name))
            sb.append(""","kind":""")
            sb.append(jsonString(m.kind))
            sb.append(""","returnType":""")
            sb.append(jsonString(m.returnType))
            if (m.nullable) sb.append(""","nullable":true""")
            sb.append(""","visibility":""")
            sb.append(jsonString(m.visibility))
            if (m.isOverride) sb.append(""","isOverride":true""")
            if (m.isAbstract) sb.append(""","isAbstract":true""")
            if (m.params.isNotEmpty()) {
                sb.append(""","params":""")
                appendParams(sb, m.params)
            }
            if (m.annotations.isNotEmpty()) {
                sb.append(""","annotations":""")
                appendStringList(sb, m.annotations)
            }
            sb.append("}")
        }
        sb.append("]")
    }

    private fun appendParams(sb: StringBuilder, params: List<ParamPayload>) {
        sb.append("[")
        params.forEachIndexed { i, p ->
            if (i > 0) sb.append(",")
            sb.append("""{"name":""")
            sb.append(jsonString(p.name))
            sb.append(""","type":""")
            sb.append(jsonString(p.type))
            if (p.nullable) sb.append(""","nullable":true""")
            sb.append("}")
        }
        sb.append("]")
    }

    private fun appendExpressions(sb: StringBuilder, expressions: Map<String, ExpressionPayload>) {
        if (expressions.isEmpty()) {
            sb.append("{}")
            return
        }
        sb.append("{")
        expressions.entries.forEachIndexed { i, (key, expr) ->
            if (i > 0) sb.append(",")
            sb.append(jsonString(key))
            sb.append(""":{"type":""")
            sb.append(jsonString(expr.type))
            sb.append(""","nullable":""")
            sb.append(expr.nullable.toString())
            if (expr.endByte > expr.startByte) {
                sb.append(""","startByte":""")
                sb.append(expr.startByte)
                sb.append(""","endByte":""")
                sb.append(expr.endByte)
            }
            expr.callTarget?.let {
                sb.append(""","callTarget":""")
                sb.append(jsonString(it))
            }
            if (expr.callTargetResolved) sb.append(""","callTargetResolved":true""")
            if (expr.callTargetSuspend) sb.append(""","callTargetSuspend":true""")
            if (expr.annotations.isNotEmpty()) {
                sb.append(""","annotations":""")
                appendStringList(sb, expr.annotations)
            }
            sb.append("}")
        }
        sb.append("}")
    }

    private fun appendDiagnostics(sb: StringBuilder, diags: List<DiagnosticPayload>) {
        sb.append("[")
        diags.forEachIndexed { i, d ->
            if (i > 0) sb.append(",")
            sb.append("""{"severity":""")
            sb.append(jsonString(d.severity))
            sb.append(""","message":""")
            sb.append(jsonString(d.message))
            sb.append(""","line":""")
            sb.append(d.line)
            sb.append(""","column":""")
            sb.append(d.column)
            sb.append("}")
        }
        sb.append("]")
    }

    private fun appendErrors(sb: StringBuilder, errors: Map<String, String>) {
        sb.append("{")
        errors.entries.forEachIndexed { i, (path, msg) ->
            if (i > 0) sb.append(",")
            sb.append(jsonString(path))
            sb.append(":")
            sb.append(jsonString(msg))
        }
        sb.append("}")
    }

    private fun appendStringList(sb: StringBuilder, items: List<String>) {
        sb.append("[")
        items.forEachIndexed { i, s ->
            if (i > 0) sb.append(",")
            sb.append(jsonString(s))
        }
        sb.append("]")
    }

    private fun jsonString(value: String): String {
        val sb = StringBuilder(value.length + 2)
        sb.append('"')
        for (c in value) {
            when {
                c == '"' -> sb.append("\\\"")
                c == '\\' -> sb.append("\\\\")
                c == '\n' -> sb.append("\\n")
                c == '\r' -> sb.append("\\r")
                c == '\t' -> sb.append("\\t")
                c.code < 0x20 -> sb.append("\\u%04x".format(c.code))
                else -> sb.append(c)
            }
        }
        sb.append('"')
        return sb.toString()
    }
}
