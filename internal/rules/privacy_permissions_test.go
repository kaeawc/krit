package rules_test

import (
	"strings"
	"testing"
)

func TestLocationBackgroundWithoutRationale(t *testing.T) {
	t.Run("flags background location request without rationale", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    fun requestBackground() {
        requestPermissions(arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION), 100)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "shouldShowRequestPermissionRationale") {
			t.Fatalf("expected rationale guidance, got %q", findings[0].Message)
		}
	})

	t.Run("allows background location request with rationale", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    fun requestBackground() {
        if (shouldShowRequestPermissionRationale(Manifest.permission.ACCESS_BACKGROUND_LOCATION)) {
            showRationaleDialog()
        }
        requestPermissions(arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION), 100)
    }

    fun showRationaleDialog() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("ignores non-background location permissions", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    fun requestForeground() {
        requestPermissions(arrayOf(Manifest.permission.ACCESS_FINE_LOCATION), 100)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestScreenshotNotBlockedOnLoginScreen(t *testing.T) {
	t.Run("flags login activity without FLAG_SECURE", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

open class AppCompatActivity {
    open fun onCreate(savedInstanceState: Any?) {}
}

class LoginActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Any?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.login)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows login activity with FLAG_SECURE", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

open class AppCompatActivity {
    open fun onCreate(savedInstanceState: Any?) {}
}

class LoginActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Any?) {
        super.onCreate(savedInstanceState)
        window.setFlags(FLAG_SECURE, FLAG_SECURE)
        setContentView(R.layout.login)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("flags payment composable without screenshot block", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

@Composable
fun PaymentScreen() {
    Column {
        Text("Enter card details")
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows composable with ScreenshotBlocker", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

@Composable
fun PaymentScreen() {
    Column(modifier = Modifier.then(ScreenshotBlocker)) {
        Text("Enter card details")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("ignores preview composables", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

annotation class Composable
annotation class Preview

@Composable
@Preview
fun PaymentScreenPreview() {
    Text("Preview only")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("ignores shipping pin substring", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

annotation class Composable

@Composable
fun ShippingAddressView() {
    Text("Address")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("ignores generic card composables without payment evidence", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

annotation class Composable

@Composable
fun RewardCard() {
    Text("Reward")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("ignores stored card helper composable", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

annotation class Composable
class StoredCard

@Composable
fun KSCardElement(card: StoredCard) {
    Text(card.toString())
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("ignores non-sensitive class names", func(t *testing.T) {
		findings := runRuleByName(t, "ScreenshotNotBlockedOnLoginScreen", `
package test

open class AppCompatActivity

class HomeActivity : AppCompatActivity() {
    fun onCreate() {
        setContentView(R.layout.home)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestClipboardOnSensitiveInputType(t *testing.T) {
	t.Run("flags setPrimaryClip from password variable", func(t *testing.T) {
		findings := runRuleByName(t, "ClipboardOnSensitiveInputType", `
package test

class LoginActivity {
    fun copyPassword(clipboardManager: Any, pwd: Any) {
        clipboardManager.setPrimaryClip(ClipData.newPlainText("", pwd.text))
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows setPrimaryClip from non-sensitive variable", func(t *testing.T) {
		findings := runRuleByName(t, "ClipboardOnSensitiveInputType", `
package test

class ShareActivity {
    fun copyLink(clipboardManager: Any, link: String) {
        clipboardManager.setPrimaryClip(ClipData.newPlainText("", link))
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestBiometricAuthNotFallingBackToDeviceCredential(t *testing.T) {
	t.Run("flags inline prompt builder without device credential fallback", func(t *testing.T) {
		findings := runRuleByName(t, "BiometricAuthNotFallingBackToDeviceCredential", `
package test

fun unlock(activity: FragmentActivity, executor: Executor, callback: BiometricPrompt.AuthenticationCallback) {
    BiometricPrompt(activity, executor, callback)
        .authenticate(PromptInfo.Builder().setTitle("Unlock").build())
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("allows inline prompt builder with allowed authenticators fallback", func(t *testing.T) {
		findings := runRuleByName(t, "BiometricAuthNotFallingBackToDeviceCredential", `
package test

fun unlock(activity: FragmentActivity, executor: Executor, callback: BiometricPrompt.AuthenticationCallback) {
    BiometricPrompt(activity, executor, callback).authenticate(
        PromptInfo.Builder()
            .setAllowedAuthenticators(BIOMETRIC_STRONG or DEVICE_CREDENTIAL)
            .setTitle("Unlock")
            .build()
    )
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("allows inline prompt builder with setDeviceCredentialAllowed true", func(t *testing.T) {
		findings := runRuleByName(t, "BiometricAuthNotFallingBackToDeviceCredential", `
package test

fun unlock(activity: FragmentActivity, executor: Executor, callback: BiometricPrompt.AuthenticationCallback) {
    BiometricPrompt(activity, executor, callback).authenticate(
        PromptInfo.Builder()
            .setDeviceCredentialAllowed(true)
            .setTitle("Unlock")
            .build()
    )
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAdMobInitializedBeforeConsent(t *testing.T) {
	t.Run("flags initialize before consent in application onCreate", func(t *testing.T) {
		findings := runRuleByName(t, "AdMobInitializedBeforeConsent", `
package test

open class Application {
    open fun onCreate() {}
}

object MobileAds {
    fun initialize(application: Application) {}
}

class ConsentInformation {
    fun requestConsentInfoUpdate() {}
}

class App : Application() {
    private val consentInformation = ConsentInformation()

    override fun onCreate() {
        super.onCreate()
        MobileAds.initialize(this)
        consentInformation.requestConsentInfoUpdate()
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Request consent info") {
			t.Fatalf("expected consent guidance, got %q", findings[0].Message)
		}
	})

	t.Run("allows initialize after earlier consent update in application onCreate", func(t *testing.T) {
		findings := runRuleByName(t, "AdMobInitializedBeforeConsent", `
package test

open class Application {
    open fun onCreate() {}
}

object MobileAds {
    fun initialize(application: Application) {}
}

class ConsentInformation {
    fun requestConsentInfoUpdate() {}
}

class App : Application() {
    private val consentInformation = ConsentInformation()

    override fun onCreate() {
        super.onCreate()
        consentInformation.requestConsentInfoUpdate()
        MobileAds.initialize(this)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("ignores initialize outside application onCreate", func(t *testing.T) {
		findings := runRuleByName(t, "AdMobInitializedBeforeConsent", `
package test

open class Activity

object MobileAds {
    fun initialize(activity: Activity) {}
}

class ConsentInformation {
    fun requestConsentInfoUpdate() {}
}

class MainActivity : Activity() {
    private val consentInformation = ConsentInformation()

    fun onCreate() {
        MobileAds.initialize(this)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestContactsAccessWithoutPermissionUi(t *testing.T) {
	t.Run("flags contacts query outside permission callback", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestContactsPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            println("permission granted")
        }
    }

    fun loadContacts() {
        requestContactsPermission.launch(Manifest.permission.READ_CONTACTS)
        resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "RequestPermission") {
			t.Fatalf("expected RequestPermission guidance, got %q", findings[0].Message)
		}
	})

	t.Run("allows contacts query in request permission callback", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestContactsPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
        }
    }

    fun loadContacts() {
        requestContactsPermission.launch(Manifest.permission.READ_CONTACTS)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("allows contacts query when launcher uses same-file string constant", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

private const val CONTACTS_PERMISSION = "android.permission.READ_CONTACTS"

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestContactsPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
        }
    }

    fun loadContacts() {
        requestContactsPermission.launch(CONTACTS_PERMISSION)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("does not treat scope-wide READ_CONTACTS mention as permission path", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

private const val UNUSED_CONTACTS_PERMISSION = Manifest.permission.READ_CONTACTS

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestCalendarPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
        }
    }

    fun loadContacts() {
        requestCalendarPermission.launch(Manifest.permission.READ_CALENDAR)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("does not treat unresolved READ_CONTACTS identifier as permission", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestContactsPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
        }
    }

    fun loadContacts(READ_CONTACTS: String) {
        requestContactsPermission.launch(READ_CONTACTS)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("does not treat unresolved wrapper call as permission", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestContactsPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
        }
    }

    fun loadContacts() {
        requestContactsPermission.launch(selectPermission(Manifest.permission.READ_CONTACTS, Manifest.permission.READ_CALENDAR))
    }

    fun selectPermission(primary: String, fallback: String): String = fallback
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("does not resolve receiver type from nested declaration", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

class ContactsScreen(private val resolver: Any) {
    fun loadContacts() {
        fun nested(resolver: ContentResolver) {}
        resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("ignores unqualified local query function", func(t *testing.T) {
		findings := runRuleByName(t, "ContactsAccessWithoutPermissionUi", `
package test

class ContactsScreen {
    fun query(uri: Any, a: Any?, b: Any?, c: Any?, d: Any?) {}

    fun loadContacts() {
        query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
