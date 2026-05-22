package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Pins the JSON-RPC dispatch path for the oracle commands (`analyze`,
 * `analyzeAll`) end-to-end through `handleRequestLine` — the same entry
 * point both the stdio and TCP daemon loops use. Field-level FIR
 * projection lives in `OracleResponseTest`; this file's job is the
 * routing contract.
 */
class OracleDispatchTest {

    private val session = AnalysisSession(emptyList(), emptyList())

    @Test
    fun analyzeCommandRoutesToOracleResponseBuilder() {
        val request = """{"id":11,"command":"analyze"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":11,"result":{"""), response)
        assertTrue(""""files":{}""" in response, response)
        assertTrue(""""dependencies":{}""" in response, response)
        assertFalse("Unknown command" in response, response)
    }

    @Test
    fun analyzeAllCommandRoutesToOracleResponseBuilder() {
        val request = """{"id":12,"command":"analyzeAll"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":12,"result":{"""), response)
        assertTrue(""""files":{}""" in response, response)
        assertTrue(""""dependencies":{}""" in response, response)
    }

    @Test
    fun analyzeFilesCommandRoutesToOracleResponseBuilder() {
        val request = """{"id":13,"command":"analyzeFiles"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":13,"result":{"""), response)
        assertTrue(""""files":{}""" in response, response)
        assertTrue(""""dependencies":{}""" in response, response)
    }

    @Test
    fun analyzeWithDepsCommandUsesFlatEnvelopeWithCacheDeps() {
        // analyzeWithDeps uses krit-types' flat envelope shape:
        // `result` / `errors` / `cacheDeps` as siblings. The
        // `cacheDeps` field always carries the krit-types preamble
        // (`version`, `approximation`) so the Go-side client can
        // detect the new-protocol daemon even when no per-file deps
        // have been recorded.
        val request = """{"id":14,"command":"analyzeWithDeps"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":14,"result":{"""), response)
        assertTrue(""""cacheDeps":{"version":1,"approximation":"symbol-resolved-sources","files":{},"crashed":{}}""" in response, response)
    }

    @Test
    fun analyzeFileWithMissingPathReturnsError() {
        val request = """{"id":21,"command":"analyzeFile"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(""""error":"analyzeFile requires path"""" in response, response)
    }

    @Test
    fun analyzeFileWithNoPluginsReturnsEmptyFindings() {
        // No plugin jars loaded → no rules to run → response carries
        // an empty findings array. The handler short-circuits before
        // touching K2 / PSI.
        val request = """{"id":22,"command":"analyzeFile","path":"/tmp/x.kt","source":"fun x() {}"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":22,"result":{"findings":["""), response)
        assertFalse(""""error":""" in response, response)
    }

    @Test
    fun listPluginsCommandReturnsEmptyRulesWhenNoJarsProvided() {
        // Zero pluginJars → the registry has nothing to load, and the
        // response carries an empty `rules` array. The shape mirrors
        // krit-types' buildListPluginsResponse so a single Go-side
        // client parses either backend's payload with one struct.
        val request = """{"id":20,"command":"listPlugins"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":20,"result":{"rules":["""), response)
        assertFalse(""""error":""" in response, response)
    }

    @Test
    fun analyzeResponseIsNotAnErrorEnvelope() {
        // Belt-and-suspenders: an `else` clause that returns
        // `{"id":...,"error":"Unknown command:..."}` is one accidental
        // dispatch ordering bug away from regressing this. Confirm the
        // response has no `error` field at the top level for any
        // oracle command.
        for (cmd in listOf("analyze", "analyzeAll", "analyzeFiles", "analyzeWithDeps")) {
            val response =
                (handleRequestLine("""{"id":1,"command":"$cmd"}""", session, 0L)
                    as RequestResult.Response).json
            assertFalse(""""error":""" in response, "$cmd should not produce an error envelope: $response")
        }
    }

    @Test
    fun unknownOracleCommandStillProducesUnknownCommandError() {
        // Negative: a typo'd command name (e.g. `analyzz`) must NOT
        // fall through to the oracle handler. Pins the dispatch's
        // exact-match behavior so additions like `analyzeWithDeps` /
        // `analyzeFiles` in later PRs don't silently absorb typos.
        val request = """{"id":99,"command":"analyzz"}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(""""error":"Unknown command:""" in response, response)
        assertEquals(true, response.contains("analyzz"), response)
    }

    @Test
    fun methodFieldIsAcceptedAsCommandAlias() {
        // Regression: internal/oracle/daemon.go sends requests as
        // {"id":N,"method":"analyzeWithDeps","params":{...}} when it
        // routes small miss requests through the persistent daemon.
        // Before this fix, parseRequest threw on the missing "command"
        // field, the daemon error-replied with id=null, and the Go side
        // saw a response ID mismatch and fell back to a fresh one-shot
        // JVM — defeating the daemon optimization entirely.
        val request = """{"id":21,"method":"analyzeAll","params":{}}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":21,"result":{"""), response)
        assertFalse(""""error":""" in response, response)
    }

    @Test
    fun methodFieldRoutesAnalyzeWithDepsCorrectly() {
        // Pin the exact path the oracle.Daemon takes most often: the
        // analyzeWithDeps round-trip used by small-miss warm reruns.
        val request = """{"id":22,"method":"analyzeWithDeps","params":{"files":[]}}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        val response = (result as RequestResult.Response).json
        assertTrue(response.startsWith("""{"id":22,"result":{"""), response)
        assertTrue(""""cacheDeps":""" in response, response)
    }

    @Test
    fun missingBothCommandAndMethodStillErrors() {
        // Negative guard: if neither field is present, parseRequest
        // must still raise so the daemon never silently processes a
        // request with no routing.
        val request = """{"id":23}"""
        val result = handleRequestLine(request, session, startTime = 0L)
        assertTrue(result is RequestResult.ParseError, "expected ParseError, got $result")
    }

    @Test
    fun emptyRequestSourceDirsReusesExistingSession() {
        // Regression for the warm-rerun session-rebuild storm: the
        // oracle.Daemon path sends analyzeWithDeps with `files` only;
        // it never re-ships `sourceDirs` (set once at daemon launch
        // via `--sources …`). Before this fix, comparing an empty
        // request.sourceDirs to the session's real source roots
        // mismatched on every call, triggering a full K2 FIR session
        // rebuild per request (10-40 s on a 16 k-file repo).
        val sessionWithSources = AnalysisSession(listOf("/repo/src/main"), listOf("/lib.jar"))
        val request = CheckRequest(
            id = 1,
            command = "analyzeWithDeps",
            sourceDirs = emptyList(),
            classpath = emptyList(),
        )
        assertFalse(
            sessionNeedsRebuild(request, sessionWithSources),
            "empty request.sourceDirs must reuse the existing session — otherwise every persistent-daemon call rebuilds",
        )
    }

    @Test
    fun nonEmptyDifferentSourceDirsStillForcesRebuild() {
        // Symmetric guard: an explicit sourceDirs that differs from
        // the session is still a real retarget, must still rebuild.
        val sessionWithSources = AnalysisSession(listOf("/repo/src/main"), emptyList())
        val request = CheckRequest(
            id = 1,
            command = "analyzeWithDeps",
            sourceDirs = listOf("/different/src"),
            classpath = emptyList(),
        )
        assertTrue(
            sessionNeedsRebuild(request, sessionWithSources),
            "explicit sourceDirs mismatch must rebuild — otherwise retargeting silently no-ops",
        )
    }

    @Test
    fun matchingExplicitSourceDirsReusesSession() {
        // Same-sourceDirs request must NOT rebuild — that case predates
        // the empty-fallback change and should still hold.
        val sessionWithSources = AnalysisSession(listOf("/repo/src/main"), emptyList())
        val request = CheckRequest(
            id = 1,
            command = "analyzeWithDeps",
            sourceDirs = listOf("/repo/src/main"),
            classpath = emptyList(),
        )
        assertFalse(sessionNeedsRebuild(request, sessionWithSources))
    }
}
