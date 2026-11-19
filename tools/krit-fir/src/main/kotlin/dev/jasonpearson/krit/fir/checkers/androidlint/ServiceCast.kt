package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirTypeOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.name.Name

/**
 * Flags `getSystemService(X_SERVICE) as T` (and `as?`) where `T` is not the manager that
 * `X_SERVICE` returns, for example `getSystemService(Context.ALARM_SERVICE) as PowerManager`.
 *
 * The call must resolve to a `getSystemService` member of `android.content.Context` or a subtype, and
 * the first argument must resolve to a `Context` constant listed in [expectedManager]. The cast
 * target is compared by the qualified name of its expanded class, as Android Lint compares it, with
 * the simple names Go prints in the message (`Any` is printed as `Object`). A cast between the two
 * `ClipboardManager` classes is accepted, as Lint accepts it.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Go compares the written name, so a type alias of the right manager is reported and a cast to a
 *   type parameter is reported. Here the alias expands to the manager and a type parameter has no
 *   class, so neither is reported.
 * - Go reads no cast target from a nullable type (`as AlarmManager?`). Here the target class is read
 *   through the nullability, so a wrong nullable cast is reported.
 * - Go matches the constant by its last identifier and the call by name. Here a same-named constant
 *   or `getSystemService` that is not the platform one is not reported.
 * - Go compares the last identifier of the written name, so a class of another package with the
 *   manager's simple name (`androidx.mediarouter.media.MediaRouter` for `MEDIA_ROUTER_SERVICE`) is not
 *   reported. That cast fails at run time, so it is reported here with both qualified names, as Lint
 *   reports it.
 */
internal object ServiceCast : FirTypeOperatorCallChecker(MppCheckerKind.Common), FirRule {
    private const val KOTLIN_ANY = "kotlin.Any"

    override val ruleId = "ServiceCast"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val typeOperatorCallCheckers = setOf(ServiceCast)
    }

    private val contextClass = classIdOf("android.content", "Context")
    private val getSystemService = Name.identifier("getSystemService")

    // Service constants of android.content.Context and the manager class each returns, as Android Lint maps them.
    private val expectedManager = mapOf(
        "ACCESSIBILITY_SERVICE" to "android.view.accessibility.AccessibilityManager",
        "ACCOUNT_SERVICE" to "android.accounts.AccountManager",
        "ACTIVITY_SERVICE" to "android.app.ActivityManager",
        "ALARM_SERVICE" to "android.app.AlarmManager",
        "APP_OPS_SERVICE" to "android.app.AppOpsManager",
        "APP_SEARCH_SERVICE" to "android.app.appsearch.AppSearchManager",
        "APPWIDGET_SERVICE" to "android.appwidget.AppWidgetManager",
        "AUDIO_SERVICE" to "android.media.AudioManager",
        "BATTERY_SERVICE" to "android.os.BatteryManager",
        "BIOMETRIC_SERVICE" to "android.hardware.biometrics.BiometricManager",
        "BLOB_STORE_SERVICE" to "android.app.blob.BlobStoreManager",
        "BLUETOOTH_SERVICE" to "android.bluetooth.BluetoothManager",
        "CAMERA_SERVICE" to "android.hardware.camera2.CameraManager",
        "CAPTIONING_SERVICE" to "android.view.accessibility.CaptioningManager",
        "CARRIER_CONFIG_SERVICE" to "android.telephony.CarrierConfigManager",
        "CLIPBOARD_SERVICE" to "android.text.ClipboardManager",
        "COMPANION_DEVICE_SERVICE" to "android.companion.CompanionDeviceManager",
        "CONNECTIVITY_DIAGNOSTICS_SERVICE" to "android.net.ConnectivityDiagnosticsManager",
        "CONNECTIVITY_SERVICE" to "android.net.ConnectivityManager",
        "CONSUMER_IR_SERVICE" to "android.hardware.ConsumerIrManager",
        "CONTACT_KEYS_SERVICE" to "android.provider.E2eeContactKeysManager",
        "CREDENTIAL_SERVICE" to "android.credentials.CredentialManager",
        "CROSS_PROFILE_APPS_SERVICE" to "android.content.pm.CrossProfileApps",
        "DEVICE_LOCK_SERVICE" to "android.devicelock.DeviceLockManager",
        "DEVICE_POLICY_SERVICE" to "android.app.admin.DevicePolicyManager",
        "DISPLAY_HASH_SERVICE" to "android.view.displayhash.DisplayHashManager",
        "DISPLAY_SERVICE" to "android.hardware.display.DisplayManager",
        "DOMAIN_VERIFICATION_SERVICE" to "android.content.pm.verify.domain.DomainVerificationManager",
        "DOWNLOAD_SERVICE" to "android.app.DownloadManager",
        "DROPBOX_SERVICE" to "android.os.DropBoxManager",
        "EUICC_SERVICE" to "android.telephony.euicc.EuiccManager",
        "FILE_INTEGRITY_SERVICE" to "android.security.FileIntegrityManager",
        "FINGERPRINT_SERVICE" to "android.hardware.fingerprint.FingerprintManager",
        "GAME_SERVICE" to "android.app.GameManager",
        "GRAMMATICAL_INFLECTION_SERVICE" to "android.app.GrammaticalInflectionManager",
        "HARDWARE_PROPERTIES_SERVICE" to "android.os.HardwarePropertiesManager",
        "HEALTHCONNECT_SERVICE" to "android.health.connect.HealthConnectManager",
        "INPUT_METHOD_SERVICE" to "android.view.inputmethod.InputMethodManager",
        "INPUT_SERVICE" to "android.hardware.input.InputManager",
        "IPSEC_SERVICE" to "android.net.IpSecManager",
        "JOB_SCHEDULER_SERVICE" to "android.app.job.JobScheduler",
        "KEYGUARD_SERVICE" to "android.app.KeyguardManager",
        "LAUNCHER_APPS_SERVICE" to "android.content.pm.LauncherApps",
        "LAYOUT_INFLATER_SERVICE" to "android.view.LayoutInflater",
        "LOCALE_SERVICE" to "android.app.LocaleManager",
        "LOCATION_SERVICE" to "android.location.LocationManager",
        "MEDIA_COMMUNICATION_SERVICE" to "android.media.MediaCommunicationManager",
        "MEDIA_METRICS_SERVICE" to "android.media.metrics.MediaMetricsManager",
        "MEDIA_PROJECTION_SERVICE" to "android.media.projection.MediaProjectionManager",
        "MEDIA_ROUTER_SERVICE" to "android.media.MediaRouter",
        "MEDIA_SESSION_SERVICE" to "android.media.session.MediaSessionManager",
        "MIDI_SERVICE" to "android.media.midi.MidiManager",
        "NETWORK_STATS_SERVICE" to "android.app.usage.NetworkStatsManager",
        "NFC_SERVICE" to "android.nfc.NfcManager",
        "NOTIFICATION_SERVICE" to "android.app.NotificationManager",
        "NSD_SERVICE" to "android.net.nsd.NsdManager",
        "OVERLAY_SERVICE" to "android.content.om.OverlayManager",
        "PEOPLE_SERVICE" to "android.app.people.PeopleManager",
        "PERFORMANCE_HINT_SERVICE" to "android.os.PerformanceHintManager",
        "PERSISTENT_DATA_BLOCK_SERVICE" to "android.service.persistentdata.PersistentDataBlockManager",
        "POWER_SERVICE" to "android.os.PowerManager",
        "PRINT_SERVICE" to "android.print.PrintManager",
        "PROFILING_SERVICE" to "android.os.ProfilingManager",
        "RESTRICTIONS_SERVICE" to "android.content.RestrictionsManager",
        "ROLE_SERVICE" to "android.app.role.RoleManager",
        "SEARCH_SERVICE" to "android.app.SearchManager",
        "SECURITY_STATE_SERVICE" to "android.os.SecurityStateManager",
        "SENSOR_SERVICE" to "android.hardware.SensorManager",
        "SHORTCUT_SERVICE" to "android.content.pm.ShortcutManager",
        "STATUS_BAR_SERVICE" to "android.app.StatusBarManager",
        "STORAGE_SERVICE" to "android.os.storage.StorageManager",
        "STORAGE_STATS_SERVICE" to "android.app.usage.StorageStatsManager",
        "SYSTEM_HEALTH_SERVICE" to "android.os.health.SystemHealthManager",
        "TELECOM_SERVICE" to "android.telecom.TelecomManager",
        "TELEPHONY_IMS_SERVICE" to "android.telephony.ims.ImsManager",
        "TELEPHONY_SERVICE" to "android.telephony.TelephonyManager",
        "TELEPHONY_SUBSCRIPTION_SERVICE" to "android.telephony.SubscriptionManager",
        "TEXT_CLASSIFICATION_SERVICE" to "android.view.textclassifier.TextClassificationManager",
        "TEXT_SERVICES_MANAGER_SERVICE" to "android.view.textservice.TextServicesManager",
        "TV_AD_SERVICE" to "android.media.tv.ad.TvAdManager",
        "TV_INPUT_SERVICE" to "android.media.tv.TvInputManager",
        "TV_INTERACTIVE_APP_SERVICE" to "android.media.tv.interactive.TvInteractiveAppManager",
        "UI_MODE_SERVICE" to "android.app.UiModeManager",
        "USAGE_STATS_SERVICE" to "android.app.usage.UsageStatsManager",
        "USB_SERVICE" to "android.hardware.usb.UsbManager",
        "USER_SERVICE" to "android.os.UserManager",
        "VIBRATOR_MANAGER_SERVICE" to "android.os.VibratorManager",
        "VIBRATOR_SERVICE" to "android.os.Vibrator",
        "VIRTUAL_DEVICE_SERVICE" to "android.companion.virtual.VirtualDeviceManager",
        "VPN_MANAGEMENT_SERVICE" to "android.net.VpnManager",
        "WALLPAPER_SERVICE" to "android.app.WallpaperManager",
        "WIFI_AWARE_SERVICE" to "android.net.wifi.aware.WifiAwareManager",
        "WIFI_P2P_SERVICE" to "android.net.wifi.p2p.WifiP2pManager",
        "WIFI_RTT_RANGING_SERVICE" to "android.net.wifi.rtt.WifiRttManager",
        "WIFI_SERVICE" to "android.net.wifi.WifiManager",
        "WINDOW_SERVICE" to "android.view.WindowManager",
    )

    // android.text.ClipboardManager is the deprecated superclass of android.content.ClipboardManager, so a cast
    // between the two is safe, as Android Lint accepts it.
    private val clipboardManagers = setOf("android.content.ClipboardManager", "android.text.ClipboardManager")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        when (expression.operation) {
            FirOperation.AS, FirOperation.SAFE_AS -> Unit
            else -> return
        }
        val call = serviceCall(expression.argumentList.arguments.singleOrNull() ?: return) ?: return
        val constant = serviceConstant(call) ?: return
        val expected = expectedManager[constant] ?: return
        val castType = castTypeName(expression) ?: return
        if (castType == expected) return
        if (castType in clipboardManagers && expected in clipboardManagers) return
        // Simple names, as Go prints them; the qualified names when only the package differs, as Lint prints them.
        val castSimple = castType.substringAfterLast('.')
        val expectedSimple = expected.substringAfterLast('.')
        val (printed, printedExpected) = when {
            castType == KOTLIN_ANY -> "Object" to expectedSimple
            castSimple == expectedSimple -> castType to expected
            else -> castSimple to expectedSimple
        }
        report(expression.source ?: return, "Suspicious cast to `$printed` for a `$constant`: expected `$printedExpected`")
    }

    // The `getSystemService` call being cast, through a safe-call wrapper.
    context(context: CheckerContext)
    private fun serviceCall(operand: FirExpression): FirFunctionCall? {
        val call = (if (operand is FirSafeCallExpression) operand.selector else operand) as? FirFunctionCall ?: return null
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return null
        if (callee.name != getSystemService) return null
        val owner = callee.getContainingClassSymbol() ?: return null
        return call.takeIf { isContext(owner) }
    }

    // The containing class comes from the symbol's lookup tag, which is bound to local classes.
    context(context: CheckerContext)
    private fun isContext(symbol: FirClassLikeSymbol<*>): Boolean =
        isOrInheritsFrom(symbol, contextClass, context.session, lookupInterfaces = false)

    // The name of the `Context` constant passed as the first argument.
    private fun serviceConstant(call: FirFunctionCall): String? {
        val argument = unwrapArgument(call.argumentList.arguments.firstOrNull() ?: return null)
        val symbol = (argument as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol() ?: return null
        return symbol.callableId?.takeIf { it.classId == contextClass }?.callableName?.asString()
    }

    // Qualified name of the cast's target class, or null when it is not a class. A local class has no
    // qualified name another class can share, so its simple name stands in.
    context(context: CheckerContext)
    private fun castTypeName(expression: FirTypeOperatorCall): String? {
        val type = (expression.conversionTypeRef as? FirResolvedTypeRef)?.coneType ?: return null
        val classType = type.fullyExpandedType() as? ConeClassLikeType ?: return null
        val classId = classType.lookupTag.classId
        return if (classId.isLocal) classId.shortClassName.asString() else classId.asFqNameString()
    }
}
