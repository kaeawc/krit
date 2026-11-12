package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
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
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags `getSystemService(X_SERVICE) as T` (and `as?`) where `T` is not the manager that
 * `X_SERVICE` returns, for example `getSystemService(Context.ALARM_SERVICE) as PowerManager`.
 *
 * The call must resolve to a `getSystemService` member of `android.content.Context` or a subtype, and
 * the first argument must resolve to a `Context` constant listed in [expectedManager]. The cast
 * target is compared by the simple name of its expanded class, as Go compares the written name, with
 * the message names Go prints (`Any` is printed as `Object`).
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Go compares the written name, so a type alias of the right manager is reported and a cast to a
 *   type parameter is reported. Here the alias expands to the manager and a type parameter has no
 *   class, so neither is reported.
 * - Go reads no cast target from a nullable type (`as AlarmManager?`). Here the target class is read
 *   through the nullability, so a wrong nullable cast is reported.
 * - Go matches the constant by its last identifier and the call by name. Here a same-named constant
 *   or `getSystemService` that is not the platform one is not reported.
 */
internal object ServiceCast : FirTypeOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ServiceCast"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val typeOperatorCallCheckers = setOf(ServiceCast)
    }

    private val contextClass = ClassId(FqName("android.content"), Name.identifier("Context"))
    private val getSystemService = Name.identifier("getSystemService")
    private val kotlinAny = ClassId(FqName("kotlin"), Name.identifier("Any"))

    // Service constants of android.content.Context and the manager each returns (simple names).
    private val expectedManager = mapOf(
        "ACCESSIBILITY_SERVICE" to "AccessibilityManager",
        "ACCOUNT_SERVICE" to "AccountManager",
        "ACTIVITY_SERVICE" to "ActivityManager",
        "ALARM_SERVICE" to "AlarmManager",
        "APPWIDGET_SERVICE" to "AppWidgetManager",
        "APP_OPS_SERVICE" to "AppOpsManager",
        "APP_SEARCH_SERVICE" to "AppSearchManager",
        "AUDIO_SERVICE" to "AudioManager",
        "BATTERY_SERVICE" to "BatteryManager",
        "BIOMETRIC_SERVICE" to "BiometricManager",
        "BLOB_STORE_SERVICE" to "BlobStoreManager",
        "BLUETOOTH_SERVICE" to "BluetoothManager",
        "CAMERA_SERVICE" to "CameraManager",
        "CAPTIONING_SERVICE" to "CaptioningManager",
        "CARRIER_CONFIG_SERVICE" to "CarrierConfigManager",
        "CLIPBOARD_SERVICE" to "ClipboardManager",
        "COMPANION_DEVICE_SERVICE" to "CompanionDeviceManager",
        "CONNECTIVITY_DIAGNOSTICS_SERVICE" to "ConnectivityDiagnosticsManager",
        "CONNECTIVITY_SERVICE" to "ConnectivityManager",
        "CONSUMER_IR_SERVICE" to "ConsumerIrManager",
        "CONTACT_KEYS_SERVICE" to "E2eeContactKeysManager",
        "CREDENTIAL_SERVICE" to "CredentialManager",
        "CROSS_PROFILE_APPS_SERVICE" to "CrossProfileApps",
        "DEVICE_LOCK_SERVICE" to "DeviceLockManager",
        "DEVICE_POLICY_SERVICE" to "DevicePolicyManager",
        "DISPLAY_HASH_SERVICE" to "DisplayHashManager",
        "DISPLAY_SERVICE" to "DisplayManager",
        "DOMAIN_VERIFICATION_SERVICE" to "DomainVerificationManager",
        "DOWNLOAD_SERVICE" to "DownloadManager",
        "DROPBOX_SERVICE" to "DropBoxManager",
        "EUICC_SERVICE" to "EuiccManager",
        "FILE_INTEGRITY_SERVICE" to "FileIntegrityManager",
        "FINGERPRINT_SERVICE" to "FingerprintManager",
        "GAME_SERVICE" to "GameManager",
        "GRAMMATICAL_INFLECTION_SERVICE" to "GrammaticalInflectionManager",
        "HARDWARE_PROPERTIES_SERVICE" to "HardwarePropertiesManager",
        "HEALTHCONNECT_SERVICE" to "HealthConnectManager",
        "INPUT_METHOD_SERVICE" to "InputMethodManager",
        "INPUT_SERVICE" to "InputManager",
        "IPSEC_SERVICE" to "IpSecManager",
        "JOB_SCHEDULER_SERVICE" to "JobScheduler",
        "KEYGUARD_SERVICE" to "KeyguardManager",
        "LAUNCHER_APPS_SERVICE" to "LauncherApps",
        "LAYOUT_INFLATER_SERVICE" to "LayoutInflater",
        "LOCALE_SERVICE" to "LocaleManager",
        "LOCATION_SERVICE" to "LocationManager",
        "MEDIA_COMMUNICATION_SERVICE" to "MediaCommunicationManager",
        "MEDIA_METRICS_SERVICE" to "MediaMetricsManager",
        "MEDIA_PROJECTION_SERVICE" to "MediaProjectionManager",
        "MEDIA_ROUTER_SERVICE" to "MediaRouter",
        "MEDIA_SESSION_SERVICE" to "MediaSessionManager",
        "MIDI_SERVICE" to "MidiManager",
        "NETWORK_STATS_SERVICE" to "NetworkStatsManager",
        "NFC_SERVICE" to "NfcManager",
        "NOTIFICATION_SERVICE" to "NotificationManager",
        "NSD_SERVICE" to "NsdManager",
        "OVERLAY_SERVICE" to "OverlayManager",
        "PEOPLE_SERVICE" to "PeopleManager",
        "PERFORMANCE_HINT_SERVICE" to "PerformanceHintManager",
        "PERSISTENT_DATA_BLOCK_SERVICE" to "PersistentDataBlockManager",
        "POWER_SERVICE" to "PowerManager",
        "PRINT_SERVICE" to "PrintManager",
        "PROFILING_SERVICE" to "ProfilingManager",
        "RESTRICTIONS_SERVICE" to "RestrictionsManager",
        "ROLE_SERVICE" to "RoleManager",
        "SEARCH_SERVICE" to "SearchManager",
        "SECURITY_STATE_SERVICE" to "SecurityStateManager",
        "SENSOR_SERVICE" to "SensorManager",
        "SHORTCUT_SERVICE" to "ShortcutManager",
        "STATUS_BAR_SERVICE" to "StatusBarManager",
        "STORAGE_SERVICE" to "StorageManager",
        "STORAGE_STATS_SERVICE" to "StorageStatsManager",
        "SYSTEM_HEALTH_SERVICE" to "SystemHealthManager",
        "TELECOM_SERVICE" to "TelecomManager",
        "TELEPHONY_IMS_SERVICE" to "ImsManager",
        "TELEPHONY_SERVICE" to "TelephonyManager",
        "TELEPHONY_SUBSCRIPTION_SERVICE" to "SubscriptionManager",
        "TEXT_CLASSIFICATION_SERVICE" to "TextClassificationManager",
        "TEXT_SERVICES_MANAGER_SERVICE" to "TextServicesManager",
        "TV_AD_SERVICE" to "TvAdManager",
        "TV_INPUT_SERVICE" to "TvInputManager",
        "TV_INTERACTIVE_APP_SERVICE" to "TvInteractiveAppManager",
        "UI_MODE_SERVICE" to "UiModeManager",
        "USAGE_STATS_SERVICE" to "UsageStatsManager",
        "USB_SERVICE" to "UsbManager",
        "USER_SERVICE" to "UserManager",
        "VIBRATOR_MANAGER_SERVICE" to "VibratorManager",
        "VIBRATOR_SERVICE" to "Vibrator",
        "VIRTUAL_DEVICE_SERVICE" to "VirtualDeviceManager",
        "VPN_MANAGEMENT_SERVICE" to "VpnManager",
        "WALLPAPER_SERVICE" to "WallpaperManager",
        "WIFI_AWARE_SERVICE" to "WifiAwareManager",
        "WIFI_P2P_SERVICE" to "WifiP2pManager",
        "WIFI_RTT_RANGING_SERVICE" to "WifiRttManager",
        "WIFI_SERVICE" to "WifiManager",
        "WINDOW_SERVICE" to "WindowManager",
    )

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
        val printed = if (castType == "Any") "Object" else castType
        report(expression.source ?: return, "Suspicious cast to `$printed` for a `$constant`: expected `$expected`")
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
        symbol.classId == contextClass ||
            lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == contextClass }

    // The name of the `Context` constant passed as the first argument.
    private fun serviceConstant(call: FirFunctionCall): String? {
        val argument = unwrapArgument(call.argumentList.arguments.firstOrNull() ?: return null)
        val symbol = (argument as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol() ?: return null
        return symbol.callableId?.takeIf { it.classId == contextClass }?.callableName?.asString()
    }

    // Simple name of the cast's target class, or null when it is not a class.
    context(context: CheckerContext)
    private fun castTypeName(expression: FirTypeOperatorCall): String? {
        val type = (expression.conversionTypeRef as? FirResolvedTypeRef)?.coneType ?: return null
        val classType = type.fullyExpandedType() as? ConeClassLikeType ?: return null
        val classId = classType.lookupTag.classId
        return if (classId == kotlinAny) "Any" else classId.shortClassName.asString()
    }
}
