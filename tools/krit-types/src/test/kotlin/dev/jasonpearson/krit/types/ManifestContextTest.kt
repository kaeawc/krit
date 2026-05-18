package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ManifestContextTest {

    private fun ctx(
        packageName: String? = null,
        minSdk: Int? = null,
        targetSdk: Int? = null,
        permissions: List<String> = emptyList(),
        activities: List<String> = emptyList(),
        exportedActivities: List<String> = emptyList(),
        services: List<String> = emptyList(),
        exportedServices: List<String> = emptyList(),
        receivers: List<String> = emptyList(),
        exportedReceivers: List<String> = emptyList(),
    ) = PayloadManifestContext(
        ManifestProfilePayload(
            packageName = packageName,
            minSdk = minSdk,
            targetSdk = targetSdk,
            permissions = permissions,
            activities = activities,
            exportedActivities = exportedActivities,
            services = services,
            exportedServices = exportedServices,
            receivers = receivers,
            exportedReceivers = exportedReceivers,
        ),
    )

    @Test
    fun scalarsPassThroughVerbatim() {
        val manifest = ctx(packageName = "com.acme.app", minSdk = 24, targetSdk = 34)
        assertEquals("com.acme.app", manifest.packageName)
        assertEquals(24, manifest.minSdk)
        assertEquals(34, manifest.targetSdk)
    }

    @Test
    fun absentScalarsRemainNull() {
        val manifest = ctx()
        assertNull(manifest.packageName)
        assertNull(manifest.minSdk)
        assertNull(manifest.targetSdk)
    }

    @Test
    fun hasPermissionMatchesByName() {
        val manifest = ctx(permissions = listOf("android.permission.INTERNET", "android.permission.CAMERA"))
        assertTrue(manifest.hasPermission("android.permission.INTERNET"))
        assertTrue(manifest.hasPermission("android.permission.CAMERA"))
        assertFalse(manifest.hasPermission("android.permission.WAKE_LOCK"))
    }

    @Test
    fun activityComponentsTrackNameAndExportedSubsetSeparately() {
        val manifest = ctx(
            activities = listOf("com.acme.MainActivity", "com.acme.HiddenActivity"),
            exportedActivities = listOf("com.acme.MainActivity"),
        )
        assertTrue(manifest.hasActivity("com.acme.MainActivity"))
        assertTrue(manifest.hasActivity("com.acme.HiddenActivity"))
        assertFalse(manifest.hasActivity("com.acme.MissingActivity"))
        assertTrue(manifest.isActivityExported("com.acme.MainActivity"))
        assertFalse(manifest.isActivityExported("com.acme.HiddenActivity"))
        assertFalse(manifest.isActivityExported("com.acme.MissingActivity"))
    }

    @Test
    fun serviceComponentsTrackExportedSubset() {
        val manifest = ctx(
            services = listOf("com.acme.MyService", "com.acme.OtherService"),
            exportedServices = listOf("com.acme.MyService"),
        )
        assertTrue(manifest.isServiceExported("com.acme.MyService"))
        assertFalse(manifest.isServiceExported("com.acme.OtherService"))
        assertTrue(manifest.hasService("com.acme.OtherService"))
    }

    @Test
    fun receiverComponentsTrackExportedSubset() {
        val manifest = ctx(
            receivers = listOf("com.acme.BootReceiver"),
            exportedReceivers = listOf("com.acme.BootReceiver"),
        )
        assertTrue(manifest.hasReceiver("com.acme.BootReceiver"))
        assertTrue(manifest.isReceiverExported("com.acme.BootReceiver"))
        assertFalse(manifest.hasReceiver("com.acme.Missing"))
    }

    @Test
    fun parsesManifestProfileViaParseRequest() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt","manifest":{"package":"com.acme.app","minSdk":24,"targetSdk":34,"permissions":["android.permission.INTERNET"],"activities":["com.acme.MainActivity"],"exportedActivities":["com.acme.MainActivity"],"services":[],"receivers":["com.acme.R"]}}}"""
        val req = parseRequest(json)
        val manifest = req.manifestProfile ?: error("manifestProfile must round-trip")
        assertEquals("com.acme.app", manifest.packageName)
        assertEquals(24, manifest.minSdk)
        assertEquals(34, manifest.targetSdk)
        assertEquals(listOf("android.permission.INTERNET"), manifest.permissions)
        assertEquals(listOf("com.acme.MainActivity"), manifest.activities)
        assertEquals(listOf("com.acme.MainActivity"), manifest.exportedActivities)
        assertEquals(emptyList(), manifest.services)
        assertEquals(listOf("com.acme.R"), manifest.receivers)
    }

    @Test
    fun parseRequestLeavesManifestProfileNullWhenAbsent() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt"}}"""
        assertNull(parseRequest(json).manifestProfile)
    }
}
