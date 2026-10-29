package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonParser
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

// Daemon-decoded findings must carry the same fix metadata (fixable,
// fixLevel, suggestedFixes) as the CLI JSON for the same file, or Apply Fix
// and Apply Suggestion disappear after a per-file rescan.
class KritDaemonFixMetadataTest {
    private fun fixMetadata(findings: List<KritFinding>) =
        findings.associate { it.findingId to Triple(it.fixable, it.fixLevel, it.suggestedFixes) }

    @Test
    fun `daemon fixable findings match the CLI JSON for the same file`() {
        // A real daemon's analyze-buffer reply and `krit --no-fir
        // --format=json` output for tests/fixtures/fixable/RedundantCode.kt.
        val data = Gson().fromJson(DAEMON_REDUNDANT_CODE, KritDaemonProtocol.AnalyzeBufferData::class.java)
        val fromDaemon = KritColumnarFindingsDecoder.decode(data.findings, data.fixLevels)
        val fromCli = KritJsonParser.parse(CLI_REDUNDANT_CODE).findings

        assertEquals(fixMetadata(fromCli), fixMetadata(fromDaemon))
        assertEquals(3, fromDaemon.count { it.fixable })
        assertEquals(
            setOf("UselessCallOnNotNull" to "idiomatic", "UnnecessaryApply" to "idiomatic", "ExplicitItLambdaParameter" to "cosmetic"),
            fromDaemon.filter { it.fixable }.map { it.rule to it.fixLevel }.toSet(),
        )
    }

    @Test
    fun `suggested fixes decode like the CLI JSON`() {
        // json.Marshal of a scanner.FindingColumns with suggestions, and
        // output.buildJSONFindings for the same columns.
        val fromDaemon = KritColumnarFindingsDecoder.decode(JsonParser.parseString(COLUMNS_WITH_SUGGESTIONS).asJsonObject)
        val fromCli = KritJsonParser.parse("""{"findings":$CLI_WITH_SUGGESTIONS}""").findings

        assertEquals(fixMetadata(fromCli), fixMetadata(fromDaemon))
        val suggestions = fromDaemon.first { it.rule == "DependenciesInRootProject" }.suggestedFixes
        assertEquals(listOf("move", "allow"), suggestions.map { it.id })
        assertEquals("tok", suggestions[1].applicationToken)
        assertEquals(
            KritSuggestedEdit("/repo/krit.yml", 2, 3, 10, 20, true, "x: [a]\n"),
            suggestions[1].edits.single(),
        )
        assertTrue(fromDaemon.first { it.rule == "Other" }.suggestedFixes.isEmpty())
    }

    @Test
    fun `a daemon without fix_levels still marks findings fixable`() {
        val data = Gson().fromJson(DAEMON_REDUNDANT_CODE, KritDaemonProtocol.AnalyzeBufferData::class.java)
        val findings = KritColumnarFindingsDecoder.decode(data.findings)
        val fixable = findings.filter { it.fixable }
        assertEquals(3, fixable.size)
        // The fix action then uses the default level.
        fixable.forEach { assertNull(it.fixLevel) }
    }

    @Test
    fun `a one-finding report round-trips in the CLI shape`() {
        // KritProjectService hands `krit apply-suggestion` a report built
        // from the finding the user picked; it must parse back unchanged
        // and use the CLI's field names.
        val finding = KritColumnarFindingsDecoder.decode(JsonParser.parseString(COLUMNS_WITH_SUGGESTIONS).asJsonObject)
            .first { it.rule == "DependenciesInRootProject" }
        val json = KritJsonParser.reportJson(listOf(finding))
        assertEquals(listOf(finding), KritJsonParser.parse(json).findings)
        val raw = JsonParser.parseString(json).asJsonObject.getAsJsonArray("findings")[0].asJsonObject
        listOf("file", "line", "column", "rule", "suggestedFixes").forEach { assertTrue(raw.has(it), it) }
        val edit = raw.getAsJsonArray("suggestedFixes")[1].asJsonObject.getAsJsonArray("edits")[0].asJsonObject
        listOf("targetFile", "startLine", "endLine", "startByte", "endByte", "byteMode", "replacement")
            .forEach { assertTrue(edit.has(it), it) }
    }

    private companion object {
        const val DAEMON_REDUNDANT_CODE = """{"findings": {"files": ["src/RedundantCode.kt"], "ruleSets": ["naming", "style"], "rules": ["MatchingDeclarationName", "InvalidPackageDeclaration", "UnusedVariable", "UselessCallOnNotNull", "UnnecessaryApply", "ExplicitItLambdaParameter"], "messages": ["File name 'RedundantCode' does not match the single top-level declaration 'Foo'", "Package declaration 'com.example.redundant' does not match the file's directory structure", "Local variable 'greeting' is never used.", "Local variable 'x' is never used.", "Local variable 'transform' is never used.", "Local variable 'trailing' is never used.", "Local variable 'tabbed' is never used.", "Useless call to orEmpty on non-null type. The value is already non-null.", "Unnecessary empty '.apply {}' block.", "Explicit 'it' lambda parameter is redundant. Use implicit 'it'."], "fixPool": [{"TargetFile": "", "StartLine": 0, "EndLine": 0, "Replacement": "", "StartByte": 133, "EndByte": 143, "ByteMode": true, "Safety": 0}, {"TargetFile": "", "StartLine": 0, "EndLine": 0, "Replacement": "", "StartByte": 201, "EndByte": 211, "ByteMode": true, "Safety": 0}, {"TargetFile": "", "StartLine": 0, "EndLine": 0, "Replacement": "{ ", "StartByte": 236, "EndByte": 244, "ByteMode": true, "Safety": 0}], "fileIdx": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "line": [3, 1, 6, 8, 9, 10, 11, 6, 8, 9], "col": [1, 1, 1, 1, 1, 1, 1, 24, 17, 1], "ruleSetIdx": [0, 0, 1, 1, 1, 1, 1, 1, 1, 1], "ruleIdx": [0, 1, 2, 2, 2, 2, 2, 3, 4, 5], "severityID": "AQEBAQEBAQEBAQ==", "messageIdx": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9], "confidence": "X19LS0tLS0tfSw==", "fixStart": [0, 0, 0, 0, 0, 0, 0, 1, 2, 3], "binaryFixStart": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]}, "cache_hit": true, "fix_levels": {"ExplicitItLambdaParameter": "cosmetic", "UnnecessaryApply": "idiomatic", "UselessCallOnNotNull": "idiomatic"}}"""
        const val CLI_REDUNDANT_CODE = """{"findings": [{"file": "src/RedundantCode.kt", "line": 1, "column": 1, "ruleSet": "naming", "rule": "InvalidPackageDeclaration", "severity": "warning", "message": "Package declaration 'com.example.redundant' does not match the file's directory structure", "fixable": false, "confidence": 0.95, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 3, "column": 1, "ruleSet": "naming", "rule": "MatchingDeclarationName", "severity": "warning", "message": "File name 'RedundantCode' does not match the single top-level declaration 'Foo'", "fixable": false, "confidence": 0.95, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 6, "column": 1, "ruleSet": "style", "rule": "UnusedVariable", "severity": "warning", "message": "Local variable 'greeting' is never used.", "fixable": false, "confidence": 0.75, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 6, "column": 24, "ruleSet": "style", "rule": "UselessCallOnNotNull", "severity": "warning", "message": "Useless call to orEmpty on non-null type. The value is already non-null.", "fixable": true, "fixLevel": "idiomatic", "confidence": 0.75, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 8, "column": 1, "ruleSet": "style", "rule": "UnusedVariable", "severity": "warning", "message": "Local variable 'x' is never used.", "fixable": false, "confidence": 0.75, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 8, "column": 17, "ruleSet": "style", "rule": "UnnecessaryApply", "severity": "warning", "message": "Unnecessary empty '.apply {}' block.", "fixable": true, "fixLevel": "idiomatic", "confidence": 0.95, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 9, "column": 1, "ruleSet": "style", "rule": "ExplicitItLambdaParameter", "severity": "warning", "message": "Explicit 'it' lambda parameter is redundant. Use implicit 'it'.", "fixable": true, "fixLevel": "cosmetic", "confidence": 0.75, "effort": "trivial"}, {"file": "src/RedundantCode.kt", "line": 9, "column": 1, "ruleSet": "style", "rule": "UnusedVariable", "severity": "warning", "message": "Local variable 'transform' is never used.", "fixable": false, "confidence": 0.75, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 10, "column": 1, "ruleSet": "style", "rule": "UnusedVariable", "severity": "warning", "message": "Local variable 'trailing' is never used.", "fixable": false, "confidence": 0.75, "effort": "local"}, {"file": "src/RedundantCode.kt", "line": 11, "column": 1, "ruleSet": "style", "rule": "UnusedVariable", "severity": "warning", "message": "Local variable 'tabbed' is never used.", "fixable": false, "confidence": 0.75, "effort": "local"}]}"""
        const val COLUMNS_WITH_SUGGESTIONS = """{"files":["build.gradle.kts"],"ruleSets":["supply-chain","style"],"rules":["DependenciesInRootProject","Other"],"messages":["m1","m2"],"suggestedFixPool":[{"ID":"move","Title":"Move to a module","Detail":"d1","Edits":null,"ApplicationToken":""},{"ID":"allow","Title":"Allow in krit.yml","Detail":"d2","Edits":[{"TargetFile":"/repo/krit.yml","StartLine":2,"EndLine":3,"Replacement":"x: [a]\n","StartByte":10,"EndByte":20,"ByteMode":true}],"ApplicationToken":"tok"}],"fileIdx":[0,0],"line":[4,9],"col":[5,1],"ruleSetIdx":[0,1],"ruleIdx":[0,1],"severityID":"AQI=","messageIdx":[0,1],"confidence":"WgA=","fixStart":[0,0],"binaryFixStart":[0,0],"suggestedFixStart":[1,0],"suggestedFixCount":[2,0]}"""
        const val CLI_WITH_SUGGESTIONS = """[{"file":"build.gradle.kts","line":4,"column":5,"ruleSet":"supply-chain","rule":"DependenciesInRootProject","severity":"warning","message":"m1","fixable":false,"confidence":0.9,"suggestedFixes":[{"id":"move","title":"Move to a module","detail":"d1"},{"id":"allow","title":"Allow in krit.yml","detail":"d2","edits":[{"targetFile":"/repo/krit.yml","startLine":2,"endLine":3,"startByte":10,"endByte":20,"byteMode":true,"replacement":"x: [a]\n"}],"applicationToken":"tok"}]},{"file":"build.gradle.kts","line":9,"column":1,"ruleSet":"style","rule":"Other","severity":"error","message":"m2","fixable":false}]"""
    }
}
