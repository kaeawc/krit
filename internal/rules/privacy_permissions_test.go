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

	t.Run("resolves same-file constant", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

const val BG = Manifest.permission.ACCESS_BACKGROUND_LOCATION
val PERMS = arrayOf(BG)

class LocationActivity {
    fun requestBackground() {
        requestPermissions(PERMS, 100)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	// Regression for #1611: a property whose initializer names itself, or two
	// properties that name each other, used to recurse until stack overflow.
	t.Run("terminates on self-referencing property initializer", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class Holder(permissions: Array<String>) {
    val permissions = permissions
}

class Other {
    val permissions: Array<String> = Holder(arrayOf()).permissions
}

class LocationActivity {
    fun request(permissions: Array<String>) {
        requestPermissions(permissions, 100)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("terminates on mutually-referencing properties", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class A {
    val first: Array<String> get() = second
    val second: Array<String> get() = first
    val a = b
    val b = a
}

class LocationActivity {
    fun request() {
        requestPermissions(a, 100)
        requestPermissions(first, 100)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("still resolves through a cycle-adjacent chain", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

val loop = loop
val perms = arrayOf(loop, Manifest.permission.ACCESS_BACKGROUND_LOCATION)

class LocationActivity {
    fun request() {
        requestPermissions(perms, 100)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("deeply nested argument stays linear", func(t *testing.T) {
		// Each nested binary expression used to re-walk its whole subtree,
		// so the fallback walk grew as 2^depth.
		expr := "Manifest.permission.ACCESS_FINE_LOCATION"
		for i := 0; i < 40; i++ {
			expr = "(" + expr + " + \"\")"
		}
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    fun request() {
        requestPermissions(arrayOf(`+expr+`), 100)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	// #1705: the fallback walk re-entered the resolver for every nested call
	// argument, so `"" + f("" + f(...))` grew ~2.8x per level (2m18s at 18).
	t.Run("deeply nested call arguments stay linear", func(t *testing.T) {
		for _, test := range []struct {
			leaf string
			want int
		}{
			{"Manifest.permission.ACCESS_FINE_LOCATION", 0},
			{"Manifest.permission.ACCESS_BACKGROUND_LOCATION", 1},
		} {
			expr := test.leaf
			for i := 0; i < 18; i++ {
				expr = "\"\" + f(" + expr + ")"
			}
			findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

fun f(s: String) = s

class LocationActivity {
    fun request() {
        requestPermissions(arrayOf(`+expr+`), 100)
    }
}
`)
			if len(findings) != test.want {
				t.Fatalf("leaf %s: expected %d findings, got %d: %v", test.leaf, test.want, len(findings), findings)
			}
		}
	})

	t.Run("resolves getter-only properties", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    val inline get() = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
    val ownLine: Array<String>
        get() = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)

    fun request() {
        requestPermissions(inline, 100)
        requestPermissions(ownLine, 101)
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("terminates on mutually-referencing getters in scope", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    val first: Array<String> get() = second
    val second: Array<String> get() = first
    val third: Array<String>
        get() = third

    fun request() {
        requestPermissions(first, 100)
        requestPermissions(third, 101)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("resolves members, companion constants and locals in scope", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    private val member = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)

    companion object {
        val PERMS = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
    }

    fun requestMember() { requestPermissions(member, 100) }
    fun requestCompanion() { requestPermissions(PERMS, 101) }
    fun requestLocal() {
        val local = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
        requestPermissions(local, 102)
    }
}
`)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("parameters and other classes' properties do not resolve", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class Y {
    val perms = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
}

class LocationActivity {
    val shadowed = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)

    fun requestOther(perms: Array<String>) { requestPermissions(perms, 100) }
    fun requestShadowed(shadowed: Array<String>) { requestPermissions(shadowed, 101) }
    fun requestLater() {
        requestPermissions(later, 102)
        val later = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	// #1733 review: a property inherited from a same-file supertype resolves
	// unqualified; a local declared in a nested block does not leak out.
	t.Run("resolves properties inherited from same-file supertypes", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

open class Activity
open class BaseActivity : Activity() {
    val bgPerms = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
    val bg = Manifest.permission.ACCESS_BACKGROUND_LOCATION
}

class Child : BaseActivity() {
    fun ask() {
        requestPermissions(bgPerms, 1)
    }

    fun askWithRationale() {
        if (shouldShowRequestPermissionRationale(bg)) return
        requestPermissions(arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION), 2)
    }
}

class GrandChild : Child() {
    fun ask() {
        requestPermissions(bgPerms, 3)
    }
}
`)
		if len(findings) != 2 || findings[0].Line != 12 || findings[1].Line != 23 {
			t.Fatalf("expected findings on lines 12 and 23, got %v", findings)
		}
	})

	t.Run("block-scoped locals do not leak out of their block", func(t *testing.T) {
		findings := runRuleByName(t, "LocationBackgroundWithoutRationale", `
package test

class LocationActivity {
    val perms = arrayOf(Manifest.permission.ACCESS_FINE_LOCATION)

    fun request() {
        run { val perms = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION) }
        if (true) { val perms = arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION) }
        requestPermissions(perms, 1)
    }
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
