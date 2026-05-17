package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class PluginRulesRequestTest {
    @Test
    fun parsesRuleConfigsForAnalyzeFile() {
        val json = """{"id":7,"method":"analyzeFile","params":{"jars":["/tmp/x.jar"],"path":"X.kt","source":"","ruleIds":["acme.NoTodo"],"ruleConfigs":{"acme.NoTodo":{"maxLineLength":120,"strict":true,"label":"prod","ignored":["TODO","FIXME"]}}}}"""

        val request = parseRequest(json)
        val configs = request.ruleConfigs ?: error("ruleConfigs must not be null")

        val rule = configs["acme.NoTodo"] ?: error("missing acme.NoTodo entry")
        assertEquals(120L, rule["maxLineLength"])
        assertEquals(true, rule["strict"])
        assertEquals("prod", rule["label"])
        assertEquals(listOf("TODO", "FIXME"), rule["ignored"])
    }

    @Test
    fun parseRequestLeavesRuleConfigsNullWhenAbsent() {
        val json = """{"id":3,"method":"analyzeFile","params":{"jars":["/tmp/x.jar"],"path":"X.kt","source":"","ruleIds":[]}}"""

        val request = parseRequest(json)

        assertNull(request.ruleConfigs)
    }

    @Test
    fun extractJsonNestedObjectMapHandlesNumbersAndBooleansAndNull() {
        val json = """{"outer":{"a":{"n":-1,"f":1.5,"b":false,"z":null},"b":{"s":"hi"}}}"""

        val map = extractJsonNestedObjectMap(json, "outer") ?: error("must parse")
        val a = map["a"] ?: error("missing a")
        assertEquals(-1L, a["n"])
        assertEquals(1.5, a["f"])
        assertEquals(false, a["b"])
        assertTrue(a.containsKey("z"))
        assertNull(a["z"])

        val b = map["b"] ?: error("missing b")
        assertEquals("hi", b["s"])
    }
}
