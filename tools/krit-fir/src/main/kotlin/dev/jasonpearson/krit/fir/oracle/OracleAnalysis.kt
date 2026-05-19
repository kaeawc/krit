package dev.jasonpearson.krit.fir.oracle

/**
 * Structured input for the `analyze` / `analyzeAll` RPC methods. The
 * wire shape mirrors what `internal/oracle/daemon.go`'s `Analyze(files)`
 * call sends:
 *
 * ```
 * {"id":N,"command":"analyze","files":[...],"sourceDirs":[...],"classpath":[...]}
 * ```
 *
 * `analyzeAll` is the zero-files variant — the daemon walks every
 * Kotlin file in `sourceDirs`.
 */
data class AnalyzeRequest(
    val id: Long,
    val command: String,
    val files: List<String> = emptyList(),
    val sourceDirs: List<String> = emptyList(),
    val classpath: List<String> = emptyList(),
)

/**
 * Per-file payload returned for an analyze request. Mirrors
 * `FileResult` in `tools/krit-types/src/main/kotlin/dev/jasonpearson/krit/types/Main.kt`
 * — same field names, same semantics, so the Go-side oracle client
 * parses either backend's response with one struct. All fields default
 * to empty so partially-populated payloads still serialize as
 * well-formed JSON.
 */
data class FilePayload(
    val packageName: String = "",
    val declarations: List<ClassPayload> = emptyList(),
    val expressions: Map<String, ExpressionPayload> = emptyMap(),
    val diagnostics: List<DiagnosticPayload> = emptyList(),
)

/**
 * Class / interface / object declaration. Mirrors krit-types'
 * `ClassResult`. Optional fields (`members`, `annotations`,
 * `typeParameters`, `jarPath`) default empty so partially-populated
 * payloads stay well-formed on the wire.
 */
data class ClassPayload(
    val fqn: String,
    val kind: String,
    val supertypes: List<String> = emptyList(),
    val isSealed: Boolean = false,
    val isData: Boolean = false,
    val isOpen: Boolean = false,
    val isAbstract: Boolean = false,
    val visibility: String = "public",
    val typeParameters: List<String> = emptyList(),
    val members: List<MemberPayload> = emptyList(),
    val annotations: List<String> = emptyList(),
    val jarPath: String? = null,
)

/** Member of a class — function / property / constructor. Mirrors `MemberResult`. */
data class MemberPayload(
    val name: String,
    val kind: String,
    val returnType: String,
    val nullable: Boolean = false,
    val visibility: String = "public",
    val isOverride: Boolean = false,
    val isAbstract: Boolean = false,
    val params: List<ParamPayload> = emptyList(),
    val annotations: List<String> = emptyList(),
)

/** Function / constructor parameter. Mirrors `ParamResult`. */
data class ParamPayload(
    val name: String,
    val type: String,
    val nullable: Boolean = false,
)

/**
 * Expression-level type information. Keyed by source-position string
 * in the `expressions` map. Mirrors krit-types' `ExpressionResult`.
 */
data class ExpressionPayload(
    val type: String,
    val nullable: Boolean = false,
    val startByte: Int = 0,
    val endByte: Int = 0,
    val callTarget: String? = null,
    val callTargetResolved: Boolean = false,
    val callTargetSuspend: Boolean = false,
    val annotations: List<String> = emptyList(),
)

/** Compiler diagnostic surfaced through the analyze envelope. */
data class DiagnosticPayload(
    val severity: String,
    val message: String,
    val line: Int,
    val column: Int = 1,
)

/**
 * Result of an analyze request. `files` is keyed by absolute file
 * path; `dependencies` is keyed by class FQN for cross-file class
 * dependencies the Go-side index needs.
 */
data class AnalyzeResult(
    val files: Map<String, FilePayload> = emptyMap(),
    val dependencies: Map<String, ClassPayload> = emptyMap(),
    val errors: Map<String, String> = emptyMap(),
) {
    companion object {
        /** An empty result with no per-file data and no errors. */
        val EMPTY: AnalyzeResult = AnalyzeResult()
    }
}
