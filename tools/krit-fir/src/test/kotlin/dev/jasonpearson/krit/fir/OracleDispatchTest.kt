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
    fun analyzeResponseIsNotAnErrorEnvelope() {
        // Belt-and-suspenders: an `else` clause that returns
        // `{"id":...,"error":"Unknown command:..."}` is one accidental
        // dispatch ordering bug away from regressing this. Confirm the
        // response has no `error` field at the top level for either
        // command.
        for (cmd in listOf("analyze", "analyzeAll")) {
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
}
