package dev.jasonpearson.krit.fir.plugins

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class PayloadContextsTest {

    @Test
    fun gradleContextResolvesDepVersionsAndScalars() {
        val ctx = PayloadGradleContext(
            GradleProfilePayload(
                minSdk = 21,
                targetSdk = 34,
                compileSdk = 34,
                kotlinVersion = "2.3.21",
                javaTargetVersion = "21",
                agpVersion = "8.5.0",
                deps = listOf("org.x:y:1.2.3", "junit:junit:4.13.2"),
            ),
        )
        assertEquals(21, ctx.minSdk)
        assertEquals("2.3.21", ctx.kotlinVersion)
        assertTrue(ctx.hasDependency("org.x", "y"))
        assertEquals("1.2.3", ctx.dependencyVersion("org.x", "y"))
        assertFalse(ctx.hasDependency("org.x", "missing"))
        // Malformed entries (no version) are dropped silently to keep
        // a junky payload from poisoning every other lookup.
        assertNull(ctx.dependencyVersion("missing", "thing"))
    }

    @Test
    fun manifestContextResolvesPermissionsActivitiesAndExports() {
        val ctx = PayloadManifestContext(
            ManifestProfilePayload(
                packageName = "com.acme",
                minSdk = 24, targetSdk = 34,
                permissions = listOf("android.permission.INTERNET"),
                activities = listOf("com.acme.MainActivity"),
                exportedActivities = listOf("com.acme.MainActivity"),
                services = listOf("com.acme.Sync"),
                exportedServices = emptyList(),
                receivers = listOf("com.acme.Boot"),
                exportedReceivers = emptyList(),
            ),
        )
        assertEquals("com.acme", ctx.packageName)
        assertTrue(ctx.hasPermission("android.permission.INTERNET"))
        assertTrue(ctx.hasActivity("com.acme.MainActivity"))
        assertTrue(ctx.isActivityExported("com.acme.MainActivity"))
        assertFalse(ctx.isServiceExported("com.acme.Sync"))
        assertFalse(ctx.isReceiverExported("com.acme.Boot"))
    }

    @Test
    fun resourcesContextParsesNameValuePairsAndIdentifierSets() {
        val ctx = PayloadResourcesContext(
            ResourcesProfilePayload(
                strings = listOf("app_name=Acme", "url=https://x?a=b"),
                drawables = listOf("ic_launcher"),
                layouts = listOf("activity_main"),
                colors = listOf("primary=#FF0000"),
                dimensions = listOf("gutter=16dp"),
                ids = listOf("button"),
            ),
        )
        assertEquals("Acme", ctx.stringValue("app_name"))
        // Values containing `=` round-trip cleanly — the parser
        // splits on the first `=` only.
        assertEquals("https://x?a=b", ctx.stringValue("url"))
        assertEquals("#FF0000", ctx.colorValue("primary"))
        assertEquals("16dp", ctx.dimensionValue("gutter"))
        assertTrue(ctx.hasDrawable("ic_launcher"))
        assertTrue(ctx.hasId("button"))
    }

    @Test
    fun moduleIndexParsesPathDirectoryDepsAndRoots() {
        val ctx = PayloadModuleIndexContext(
            ModulesProfilePayload(
                modules = listOf(
                    ":app|/repo/app|:lib,:util|/repo/app/src/main/kotlin",
                    ":lib|/repo/lib||/repo/lib/src/main/kotlin,/repo/lib/src/main/java",
                ),
            ),
        )
        assertEquals(listOf(":app", ":lib"), ctx.modulePaths)
        assertEquals("/repo/app", ctx.directoryOf(":app"))
        assertEquals(listOf(":lib", ":util"), ctx.dependenciesOf(":app"))
        assertEquals(emptyList(), ctx.dependenciesOf(":lib"))
        assertEquals(2, ctx.sourceRootsOf(":lib").size)
    }

    @Test
    fun crossFileContextParsesDeclarationsAndReferences() {
        val ctx = PayloadCrossFileContext(
            CrossFileProfilePayload(
                declarations = listOf(
                    "com.acme.Foo|class|/Foo.kt|10|public",
                    "com.acme.Bar|interface|/Bar.kt|3|",
                ),
                nonCommentRefsByName = listOf(
                    "Foo|/A.kt,/B.kt",
                    "Bar|/A.kt",
                ),
            ),
        )
        val foo = ctx.declarationByFqn("com.acme.Foo")!!
        assertEquals("class", foo.kind)
        assertEquals("/Foo.kt", foo.file)
        assertEquals(10, foo.line)
        assertEquals("public", foo.visibility)
        assertNull(ctx.declarationByFqn("com.acme.Bar")?.visibility)
        assertEquals(listOf("/A.kt", "/B.kt"), ctx.referenceFiles("Foo"))
        assertTrue(ctx.isReferenced("Bar"))
        assertFalse(ctx.isReferenced("Baz"))
    }
}
