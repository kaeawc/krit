package rules_test

// wearLintCase is a synthetic project and the findings Android Lint 32.4.1 reports on it, as "RuleID path:line:col".
type wearLintCase struct {
	name  string
	files map[string]string
	want  []string
}

// wearLintCases lists the projects the Wear checks are compared on.
var wearLintCases = []wearLintCase{
	{
		name: "feature_requiredFalse",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" android:required="false"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
		want: []string{"InvalidWearFeatureAttribute AndroidManifest.xml:3:60"},
	},
	{
		name: "feature_requiredTrue",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" android:required="true"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
	},
	{
		name: "feature_requiredAbsent",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" />
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
	},
	{
		name: "feature_requiredRef",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" android:required="@bool/r"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
		want: []string{"InvalidWearFeatureAttribute AndroidManifest.xml:3:60"},
	},
	{
		name: "feature_requiredUpper",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" android:required="FALSE"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
		want: []string{"InvalidWearFeatureAttribute AndroidManifest.xml:3:60"},
	},
	{
		name: "feature_requiredToolsMerge",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" android:required="false" tools:node="merge"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
		want: []string{"InvalidWearFeatureAttribute AndroidManifest.xml:3:60"},
	},
	{
		name: "standalone_valueMissing",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" />
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:5:6"},
	},
	{
		name: "standalone_valueInvalid",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="maybe"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:5:85"},
	},
	{
		name: "standalone_valueRef",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="@bool/r"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:5:85"},
	},
	{
		name: "standalone_valueTrue",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  </application>
</manifest>
`,
		},
	},
	{
		name: "standalone_valueMixedCase",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="True"/>
  </application>
</manifest>
`,
		},
	},
	{
		name: "standalone_valueEmpty",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value=""/>
  </application>
</manifest>
`,
		},
	},
	{
		name: "standalone_absent",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "standalone_outsideApplication",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:5:4"},
	},
	{
		name: "standalone_target22",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="22"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
	},
	{
		name: "standalone_target23",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="23"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "standalone_noSdk",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
	},
	{
		name: "standalone_minSdkOnly",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "standalone_codenameTarget",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="VanillaIceCream"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "standalone_notWearInvalidValue",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="x"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:85"},
	},
	{
		name: "standalone_notWearAbsent",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
	},
	{
		name: "standalone_noApplication",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
</manifest>
`,
		},
	},
	{
		name: "standalone_featureInsideApplication",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <uses-feature android:name="android.hardware.type.watch"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:3:4"},
	},
	{
		name: "recents_combinations",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".A" android:exported="true" android:excludeFromRecents="true"/>
    <activity android:name=".B" android:exported="true" android:excludeFromRecents="true" android:noHistory="false"/>
    <activity android:name=".C" android:exported="true" android:noHistory="true" android:taskAffinity=""/>
    <activity android:name=".D" android:exported="true" />
    <activity android:name=".E" android:exported="true" android:excludeFromRecents="false" android:taskAffinity=":x"/>
    <activity android:name=".F" android:exported="true" android:excludeFromRecents="true" android:noHistory="true"/>
    <activity android:name=".G" android:exported="true" android:taskAffinity=":g"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:5:5", "WearRecents AndroidManifest.xml:8:5", "WearRecents AndroidManifest.xml:9:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "recents_notWear",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <activity android:name=".A" android:exported="true" android:excludeFromRecents="true"/>
    <activity android:name=".D" android:exported="true" />
  </application>
</manifest>
`,
		},
	},
	{
		name: "recents_featureAfterApplication",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <activity android:name=".D" android:exported="true" />
  </application>
  <uses-feature android:name="android.hardware.type.watch" android:required="true"/>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:4:5", "WearStandaloneAppFlag AndroidManifest.xml:3:4"},
	},
	{
		name: "recents_featureToolsRemove",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch" tools:node="remove"/>
  <application android:label="x">
    <activity android:name=".D" android:exported="true" />
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "recents_featureInsideApplication",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <uses-feature android:name="android.hardware.type.watch"/>
    <activity android:name=".D" android:exported="true" />
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:3:4"},
	},
	{
		name: "recents_serviceAndAliasIgnored",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity-alias android:name=".Al" android:targetActivity=".A" android:exported="true"/>
    <service android:name=".S" android:exported="true"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "splash_names",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".SplashActivity" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".splashLower" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".MySplash" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name="com.example.pkg.SplashActivity" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".Main" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".Other" android:exported="true" android:label="Splash">
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:11:5", "WearRecents AndroidManifest.xml:17:5", "WearRecents AndroidManifest.xml:23:5", "WearRecents AndroidManifest.xml:29:5", "WearRecents AndroidManifest.xml:35:5", "WearRecents AndroidManifest.xml:5:5", "WearSplashScreen AndroidManifest.xml:17:5", "WearSplashScreen AndroidManifest.xml:23:5", "WearSplashScreen AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "splash_filters",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".SplashA" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.VIEW"/>
    </intent-filter>
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".SplashB" android:exported="true" >
    <intent-filter>
      <category android:name="android.intent.category.LAUNCHER"/>
      <action android:name="android.intent.action.MAIN"/>
    </intent-filter>
    </activity>
    <activity android:name=".SplashC" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.DEFAULT"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".SplashD" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.VIEW"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".SplashE" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.main"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
    <activity android:name=".SplashNoFilter" android:exported="true" />
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:14:5", "WearRecents AndroidManifest.xml:20:5", "WearRecents AndroidManifest.xml:27:5", "WearRecents AndroidManifest.xml:33:5", "WearRecents AndroidManifest.xml:39:5", "WearRecents AndroidManifest.xml:5:5", "WearSplashScreen AndroidManifest.xml:14:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "splash_alias",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity-alias android:name=".SplashAlias" android:targetActivity=".X" android:exported="true">
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity-alias>
  </application>
</manifest>
`,
		},
		want: []string{"WearSplashScreen AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "splash_minSdk31",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="31" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".SplashActivity" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "splash_minSdk30",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="30" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".SplashActivity" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:5:5", "WearSplashScreen AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "splash_noSdk",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".SplashActivity" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WearRecents AndroidManifest.xml:4:5", "WearSplashScreen AndroidManifest.xml:4:5"},
	},
	{
		name: "splash_notWear",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <activity android:name=".SplashActivity" android:exported="true" >
    <intent-filter>
      <action android:name="android.intent.action.MAIN"/>
      <category android:name="android.intent.category.LAUNCHER"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
	},
	{
		name: "editor_modes",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".E1" android:exported="true" android:launchMode="singleTask">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E2" android:exported="true" android:launchMode="standard">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E3" android:exported="true" android:launchMode="STANDARD">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E4" android:exported="true" android:launchMode="singleInstance">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E5" android:exported="true" android:launchMode="@string/lm">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E6" android:exported="true" android:launchMode=" standard">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E7" android:exported="true" >
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E8" android:exported="true" android:launchMode="">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
    <activity android:name=".E9" android:exported="true" android:launchMode="singleTop">
    <intent-filter>
      <action android:name="other.ACTION"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceEditor AndroidManifest.xml:15:58", "WatchFaceEditor AndroidManifest.xml:20:58", "WatchFaceEditor AndroidManifest.xml:20:58", "WatchFaceEditor AndroidManifest.xml:28:58", "WatchFaceEditor AndroidManifest.xml:33:58", "WatchFaceEditor AndroidManifest.xml:5:58", "WearRecents AndroidManifest.xml:10:5", "WearRecents AndroidManifest.xml:15:5", "WearRecents AndroidManifest.xml:20:5", "WearRecents AndroidManifest.xml:28:5", "WearRecents AndroidManifest.xml:33:5", "WearRecents AndroidManifest.xml:38:5", "WearRecents AndroidManifest.xml:43:5", "WearRecents AndroidManifest.xml:48:5", "WearRecents AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "editor_twoActionsOneFilter",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity android:name=".E1" android:exported="true" android:launchMode="singleTop">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceEditor AndroidManifest.xml:5:58", "WatchFaceEditor AndroidManifest.xml:5:58", "WearRecents AndroidManifest.xml:5:5", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "editor_notWear",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <activity android:name=".E1" android:exported="true" android:launchMode="singleTop">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceEditor AndroidManifest.xml:4:58"},
	},
	{
		name: "editor_aliasAndServiceIgnored",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <activity-alias android:name=".A" android:targetActivity=".X" android:exported="true" android:launchMode="singleTop">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </activity-alias>
    <service android:name=".S" android:exported="true" android:launchMode="singleTop">
    <intent-filter>
      <action android:name="androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"/>
    </intent-filter>
    </service>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_noHasCode",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeTrue",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="true">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:34", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeFalse",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="false">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeFalseUpper",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="FALSE">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:34", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodePlaceholder",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="${hc}">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:34", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeUndefinedRef",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="@bool/nope">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeRefFalse",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="@bool/f">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
			"res/values/b.xml": `<resources><bool name="f">false</bool></resources>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeRefMixed",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="@bool/f">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
			"res/values-v26/b.xml": `<resources><bool name="f">true</bool></resources>
`,
			"res/values/b.xml": `<resources><bool name="f">false</bool></resources>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:34", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeRefItem",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="@bool/g">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
			"res/values/b.xml": `<resources><item name="g" type="bool">false</item></resources>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeRefString",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="@string/f">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
			"res/values/b.xml": `<resources><string name="f">false</string></resources>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_hasCodeRefChain",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="@bool/f">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
			"res/values/b.xml": `<resources><bool name="f">@bool/g</bool><bool name="g">false</bool></resources>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:34", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_rawNoProperty",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
			"res/raw/face.xml": `<WatchFace width="450" height="450"/>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatMissingVersion AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_rawNoPropertyHasCodeFalse",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x" android:hasCode="false">
  </application>
</manifest>
`,
			"res/raw/face.xml": `<WatchFace width="450" height="450"/>
`,
		},
		want: []string{"WatchFaceFormatMissingVersion AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_rawOtherRoot",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
			"res/raw/other.xml": `<Other/>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_rawNotXml",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
			"res/raw/blob.bin": `not xml at all`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_noRaw",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_rawWithProperty",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
			"res/raw/face.xml": `<WatchFace width="450" height="450"/>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionMissingValue",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" />
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatMissingVersion AndroidManifest.xml:5:6", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionInvalid",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="abc"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatInvalidVersion AndroidManifest.xml:5:71", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionRef",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="@integer/v"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatInvalidVersion AndroidManifest.xml:5:71", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionPlaceholder",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="${ver}"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionPlaceholderMixed",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="1${ver}"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionNegative",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="-3"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionPlus",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="+2"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionSpace",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value=" 2"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatInvalidVersion AndroidManifest.xml:5:71", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionOverflow",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="99999999999"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatInvalidVersion AndroidManifest.xml:5:71", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_versionDecimal",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="1.0"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatDeclaresHasNoCode AndroidManifest.xml:4:4", "WatchFaceFormatInvalidVersion AndroidManifest.xml:5:71", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "wff_notWearProperty",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" android:value="abc"/>
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatInvalidVersion AndroidManifest.xml:4:71"},
	},
	{
		name: "wff_notWearMissingValue",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
    <property android:name="com.google.wear.watchface.format.version" />
  </application>
</manifest>
`,
		},
		want: []string{"WatchFaceFormatMissingVersion AndroidManifest.xml:4:6"},
	},
	{
		name: "wff_notWearHasCode",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x" android:hasCode="true">
    <property android:name="com.google.wear.watchface.format.version" android:value="1"/>
  </application>
</manifest>
`,
		},
	},
	{
		name: "wff_metaDataIsNotProperty",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.wear.watchface.format.version" android:value="x"/>
  </application>
</manifest>
`,
		},
		want: []string{"WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "password_wear",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
  </application>
</manifest>
`,
			"res/layout-round/pw.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="text|textVisiblePassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="numberPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textWebPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPasswordX"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textwebpassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textEmailAddress"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="@integer/it"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="textPassword"/>
  <androidx.appcompat.widget.AppCompatEditText android:layout_width="wrap_content" android:layout_height="wrap_content"
      android:autofillHints="password" android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      tools:ignore="WearPasswordInput" android:inputType="textPassword"/>
  <FrameLayout android:layout_width="match_parent" android:layout_height="match_parent" tools:ignore="WearPasswordInput">
    <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
        android:inputType="textPassword"/>
  </FrameLayout>
</LinearLayout>
`,
			"res/layout/pw.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="text|textVisiblePassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="numberPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textWebPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPasswordX"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textwebpassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textEmailAddress"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="@integer/it"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="textPassword"/>
  <androidx.appcompat.widget.AppCompatEditText android:layout_width="wrap_content" android:layout_height="wrap_content"
      android:autofillHints="password" android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      tools:ignore="WearPasswordInput" android:inputType="textPassword"/>
  <FrameLayout android:layout_width="match_parent" android:layout_height="match_parent" tools:ignore="WearPasswordInput">
    <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
        android:inputType="textPassword"/>
  </FrameLayout>
</LinearLayout>
`,
			"res/menu/m.xml": `<menu xmlns:android="http://schemas.android.com/apk/res/android"/>
`,
		},
		want: []string{"WearPasswordInput res/layout-round/pw.xml:10:7", "WearPasswordInput res/layout-round/pw.xml:12:7", "WearPasswordInput res/layout-round/pw.xml:4:7", "WearPasswordInput res/layout-round/pw.xml:6:7", "WearPasswordInput res/layout-round/pw.xml:8:7", "WearPasswordInput res/layout/pw.xml:10:7", "WearPasswordInput res/layout/pw.xml:12:7", "WearPasswordInput res/layout/pw.xml:4:7", "WearPasswordInput res/layout/pw.xml:6:7", "WearPasswordInput res/layout/pw.xml:8:7", "WearStandaloneAppFlag AndroidManifest.xml:4:4"},
	},
	{
		name: "password_notWear",
		files: map[string]string{
			"AndroidManifest.xml": `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <application android:label="x">
  </application>
</manifest>
`,
			"res/layout-round/pw.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="text|textVisiblePassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="numberPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textWebPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPasswordX"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textwebpassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textEmailAddress"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="@integer/it"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="textPassword"/>
  <androidx.appcompat.widget.AppCompatEditText android:layout_width="wrap_content" android:layout_height="wrap_content"
      android:autofillHints="password" android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      tools:ignore="WearPasswordInput" android:inputType="textPassword"/>
  <FrameLayout android:layout_width="match_parent" android:layout_height="match_parent" tools:ignore="WearPasswordInput">
    <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
        android:inputType="textPassword"/>
  </FrameLayout>
</LinearLayout>
`,
			"res/layout/pw.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="text|textVisiblePassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="numberPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textWebPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textPasswordX"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textwebpassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="textEmailAddress"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      android:inputType="@integer/it"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="textPassword"/>
  <androidx.appcompat.widget.AppCompatEditText android:layout_width="wrap_content" android:layout_height="wrap_content"
      android:autofillHints="password" android:inputType="textPassword"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
      tools:ignore="WearPasswordInput" android:inputType="textPassword"/>
  <FrameLayout android:layout_width="match_parent" android:layout_height="match_parent" tools:ignore="WearPasswordInput">
    <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:autofillHints="password"
        android:inputType="textPassword"/>
  </FrameLayout>
</LinearLayout>
`,
			"res/menu/m.xml": `<menu xmlns:android="http://schemas.android.com/apk/res/android"/>
`,
		},
	},
}
