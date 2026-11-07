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
 * Per-file payload returned for an analyze request. Field names match
 * the struct the Go-side oracle client (internal/oracle) decodes.
 * All fields default
 * to empty so partially-populated payloads still serialize as
 * well-formed JSON.
 */
data class FilePayload(
    val packageName: String = "",
    val declarations: List<ClassPayload> = emptyList(),
    val expressions: Map<String, ExpressionPayload> = emptyMap(),
    val diagnostics: List<DiagnosticPayload> = emptyList(),
    // Lambda functional-type facts keyed by `"line:col"` of the
    // lambda's source position. True iff the lambda's resolved
    // declaration status carries the suspend modifier — either
    // because it was passed to a `suspend (...) -> R` parameter or
    // because its own inferred type is `kotlin.coroutines.SuspendFunctionN`.
    val lambdaSuspendByLineCol: Map<String, Boolean> = emptyMap(),
)

/**
 * Class / interface / object declaration. Optional fields (`members`, `annotations`,
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
 * in the `expressions` map.
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
    /** Other source files declaring the properties a stable smart cast behind this type relies on. */
    val smartCastDeclarations: List<String> = emptyList(),
)

/** True iff both payloads span the same byte range. */
internal fun ExpressionPayload.sameRange(other: ExpressionPayload): Boolean =
    startByte == other.startByte && endByte == other.endByte

/**
 * Key of an expression that starts where another recorded expression starts but ends at [endByte]:
 * "line:col@endByte". The Go side reads it through the byte-range index only.
 */
internal fun rangeKey(key: String, endByte: Int): String = "$key@$endByte"

/**
 * Compiler diagnostic surfaced through the analyze envelope. Field
 * names match the struct the Go-side oracle client decodes.
 *
 * `startByte` / `endByte` are 0-based UTF-8 byte offsets and are
 * emitted only when they describe a real range (`endByte > startByte`);
 * omitted otherwise to keep the wire payload minimal.
 *
 * `smartCastDeclarations` lists the other source files declaring the
 * properties a smart cast behind a smart-cast-dependent verdict relies on.
 * The oracle compiles every module as one, so such a smart cast is stable
 * here even when a per-module build rejects it; the caller drops the
 * verdict when one of those files belongs to another, non-friend module.
 */
data class DiagnosticPayload(
    val factoryName: String,
    val severity: String,
    val message: String,
    val line: Int,
    val col: Int = 1,
    val startByte: Int = 0,
    val endByte: Int = 0,
    val smartCastDeclarations: List<String> = emptyList(),
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
