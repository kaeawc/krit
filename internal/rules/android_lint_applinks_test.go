package rules_test

import "testing"

// appLinksManifest and its expectations come from running Android Lint 32.4.1 on the same manifest.
const appLinksManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth" android:versionCode="1" android:versionName="1">
  <application android:label="x" android:icon="@mipmap/ic">
    <activity android:name=".A0" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A1" android:exported="true">
      <intent-filter android:autoVerify="true">
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A2" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A3" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https"/>
      </intent-filter>
    </activity>
    <activity android:name=".A4" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A5" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <data android:path="/x"/>
      </intent-filter>
    </activity>
    <activity android:name=".A6" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="myapp" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A7" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com" android:mimeType="image/png"/>
      </intent-filter>
    </activity>
    <activity android:name=".A8" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
      </intent-filter>
    </activity>
    <activity android:name=".A9" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com" android:path="/x?y=1"/>
        <data android:pathPrefix="/p#f"/>
      </intent-filter>
    </activity>
    <activity android:name=".A10" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com" android:pathPattern="/x?y"/>
        <data android:pathAdvancedPattern="/x?y"/>
      </intent-filter>
    </activity>
    <activity android:name=".A11" android:exported="true">
      <intent-filter android:autoVerify="false">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="HTTPS" android:host="A.com" android:mimeType="Text/Plain"/>
      </intent-filter>
    </activity>
    <activity android:name=".A12" android:exported="true">
      <intent-filter android:autoVerify="false">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https:" android:host="a.*.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A13" android:exported="true">
      <intent-filter android:autoVerify="false">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="" android:host="" android:port=""/>
      </intent-filter>
    </activity>
    <activity android:name=".A14" android:exported="true">
      <intent-filter android:autoVerify="false">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com" android:port="70000"/>
        <data android:scheme="https" android:host="a.com" android:port="abc"/>
      </intent-filter>
    </activity>
    <activity android:name=".A15" android:exported="true">
      <intent-filter android:autoVerify="false">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
        <data android:port="80"/>
      </intent-filter>
    </activity>
    <activity android:name=".A16" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="${host}" android:path="${p}"/>
      </intent-filter>
    </activity>
    <activity android:name=".A17" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="nodot"/>
      </intent-filter>
    </activity>
    <activity android:name=".A18" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com" android:path=""/>
        <data android:pathPrefix="  "/>
      </intent-filter>
    </activity>
    <activity android:name=".A19" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com" android:path="x"/>
        <data android:pathPrefix="x"/>
        <data android:pathPattern="x"/>
        <data android:pathPattern=".*x"/>
        <data android:pathSuffix="x"/>
      </intent-filter>
    </activity>
    <activity android:name=".A20" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:port="80"/>
      </intent-filter>
    </activity>
    <activity android:name=".A21" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A22" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <data android:scheme="myapp"/>
      </intent-filter>
    </activity>
    <activity android:name=".A23" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
      </intent-filter>
    </activity>
    <activity android:name=".A24" android:exported="false">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A25" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A26" android:exported="true">
      <intent-filter android:autoVerify="false">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="http" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A27" android:exported="true">
      <intent-filter android:autoVerify="maybe">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A28" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
        <data android:scheme="myapp"/>
      </intent-filter>
    </activity>
    <activity android:name=".A29" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="@string/h"/>
      </intent-filter>
    </activity>
    <activity android:name=".A30" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.SEND"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <data android:scheme="https" android:host="a.com"/>
      </intent-filter>
    </activity>
    <activity android:name=".A31" android:exported="true">
      <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="https" android:host="a.com"/>
        <uri-relative-filter-group><data android:pathPrefix="/p"/></uri-relative-filter-group>
      </intent-filter>
    </activity>
    <activity-alias android:name=".Alias" android:targetActivity=".A0" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <data android:scheme="https" android:host="b.com"/>
      </intent-filter>
    </activity-alias>
  </application>
</manifest>
`

func TestAppLinkChecksMatchLint(t *testing.T) {
	checkManifestParity(t, appLinksManifest, map[string][]string{
		"AppLinkUrlError": {
			"13:8 VIEW action is missing, but is required for Android App Links",
			"20:7 Activity supporting ACTION_VIEW is not set as BROWSABLE",
			"20:8 BROWSABLE category and DEFAULT category are missing, but are required for Android App Links",
			"26:8 `host` attribute is missing, but is required for Android App Links",
			"38:9 At least one `scheme` must be specified",
			"42:8 Several elements/attributes (such as BROWSABLE category) required for Android App Links are missing",
			"44:9 At least one `scheme` must be specified",
			"44:9 At least one `host` must be specified",
			"48:8 `http(s)` scheme is missing, but is required for Android App Links",
			"64:8 `http(s)` scheme and `host` attribute are missing, but are required for Android App Links",
			"75:9 App link matching does not support query parameters or fragments, unless using `<uri-relative-filter-group>` (introduced in Android 15)",
			"76:9 App link matching does not support query parameters or fragments, unless using `<uri-relative-filter-group>` (introduced in Android 15)",
			"84:80 pathPattern does not support `?` as a Regex character",
			"85:44 pathAdvancedPattern does not support `?` as a Regex character",
			"93:31 Scheme matching is case sensitive and should only use lower-case characters",
			"93:52 Host matching is case sensitive and should only use lower-case characters",
			"93:77 Mime-type matching is case sensitive and should only use lower-case characters",
			"101:31 Don't include trailing colon in the `scheme` declaration",
			"101:53 The host wildcard (`*`) can only be the first character",
			"109:15 `android:scheme` cannot be empty",
			"109:33 `android:host` cannot be empty",
			"109:49 `android:port` cannot be empty",
			"117:73 not a valid port number",
			"118:73 not a valid port number",
			"127:29 The port must be specified in the same `<data>` element as the `host`",
			"143:52 Android App Links' `host` attributes must be valid web domains",
			"151:59 `android:path` cannot be empty",
			"152:15 `android:pathPrefix` cannot be empty",
			"160:73 `android:path` attribute should start with `/`, but it is `x`",
			"161:35 `android:pathPrefix` attribute should start with `/`, but it is `x`",
			"162:36 `android:pathPattern` attribute should start with `/` or `.*`, but it is `x`",
			"172:9 VIEW actions require a URI",
			"172:29 The port must be specified in the same `<data>` element as the `host`",
			"176:7 Activity supporting ACTION_VIEW is not set as BROWSABLE",
			"190:7 Missing data element",
			"196:5 Activity supporting ACTION_VIEW is not exported",
			"262:7 Activity supporting ACTION_VIEW is not set as BROWSABLE",
		},
		"AppLinkWarning": {
			"80:8 This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link.",
			"147:8 This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link.",
			"156:8 This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link.",
			"197:8 This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link.",
			"205:8 This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link.",
			"221:8 This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link.",
		},
	}, true)
}

// A scheme that is a resource reference cannot be resolved without reading resources, so the filter-level checks
// stay silent instead of guessing.
func TestAppLinkChecksSkipUnresolvedSchemeReference(t *testing.T) {
	const manifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <application>
    <activity android:name=".A" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <category android:name="android.intent.category.DEFAULT"/>
        <category android:name="android.intent.category.BROWSABLE"/>
        <data android:scheme="@string/web_scheme" android:host="a.com"/>
      </intent-filter>
    </activity>
  </application>
</manifest>
`
	checkManifestParity(t, manifest, map[string][]string{"AppLinkWarning": nil, "AppLinkUrlError": nil}, true)
}
