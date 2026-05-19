package dev.jasonpearson.krit.fir.plugins

import org.junit.jupiter.api.Test
import kotlin.test.assertTrue

class PluginResponseTest {

    @Test
    fun emptyListPluginsResponseShape() {
        val response = PluginResponse.buildListPlugins(
            id = 1,
            descriptors = emptyList(),
            diagnostics = emptyList(),
        )
        assertTrue(response.startsWith("""{"id":1,"result":{"rules":[]"""), response)
        // No diagnostics → no `"diagnostics"` field. Matches
        // krit-types' omit-when-empty rule so a healthy load surface
        // stays minimal.
        assertTrue("diagnostics" !in response, response)
    }

    @Test
    fun populatedRuleDescriptorSerializes() {
        val response = PluginResponse.buildListPlugins(
            id = 2,
            descriptors = listOf(
                PluginRuleDescriptor(
                    ruleId = "com.acme.MyRule",
                    category = "performance",
                    severity = "warning",
                    maturity = "stable",
                    languages = listOf("kotlin"),
                    needs = listOf("NEEDS_RESOLVER", "NEEDS_FIR"),
                    sdkVersion = "1.2.3",
                ),
            ),
            diagnostics = emptyList(),
        )
        assertTrue(""""ruleId":"com.acme.MyRule"""" in response, response)
        assertTrue(""""category":"performance"""" in response, response)
        assertTrue(""""needs":["NEEDS_RESOLVER","NEEDS_FIR"]""" in response, response)
        assertTrue(""""sdkVersion":"1.2.3"""" in response, response)
    }

    @Test
    fun blankSdkVersionOmitsSdkVersionField() {
        val response = PluginResponse.buildListPlugins(
            id = 3,
            descriptors = listOf(
                PluginRuleDescriptor(
                    ruleId = "x",
                    category = "custom",
                    severity = "info",
                    maturity = "experimental",
                    languages = emptyList(),
                    needs = emptyList(),
                    sdkVersion = "",
                ),
            ),
            diagnostics = emptyList(),
        )
        assertTrue("sdkVersion" !in response, response)
    }

    @Test
    fun diagnosticsSerializeWithJarLevelAndMessage() {
        val response = PluginResponse.buildListPlugins(
            id = 4,
            descriptors = emptyList(),
            diagnostics = listOf(
                PluginLoadDiagnostic(
                    jar = "/p/r.jar",
                    level = PluginLoadDiagnostic.Level.ERROR,
                    ruleSdkVersion = "1.0.0",
                    daemonSdkVersion = "2.0.0",
                    message = "major version mismatch",
                ),
            ),
        )
        assertTrue(""""diagnostics":[""" in response, response)
        assertTrue(""""jar":"/p/r.jar"""" in response, response)
        assertTrue(""""level":"error"""" in response, response)
        assertTrue(""""message":"major version mismatch"""" in response, response)
    }
}
