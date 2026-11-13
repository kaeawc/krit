package rules_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// lintGradleParityCase is a build script with the findings Android Lint 32.4.1 reports for it, per rule, by line.
type lintGradleParityCase struct {
	name, path, content string
	want                map[string][]int
}

// lintGradleParityRules are the rules checked on every case; a rule without expected lines must stay quiet.
var lintGradleParityRules = []string{
	"NotInterpolated", "BomWithoutPlatform", "AnnotationProcessorOnCompilePath", "UseOfBundledGooglePlayServices",
	"DevModeObsolete", "HighAppVersionCode", "DataBindingWithoutKapt", "AccidentalOctal", "GradlePath",
	"StringInteger", "GradleDynamicVersion", "GradleDeprecated",
}

var lintGradleParityCases = []lintGradleParityCase{
	{
		name: "groovy dependencies",
		path: "build.gradle",
		content: `apply plugin: 'com.android.application'
android {
    compileSdkVersion 34
    defaultConfig {
        minSdkVersion '21'
        targetSdkVersion "33"
        versionCode "5"
        versionName 7
        maxSdkVersion 30
    }
    buildTypes {
        release {
            minSdkVersion 18
        }
    }
    flavorDimensions "a"
    productFlavors {
        one {
            versionCode 2100000000
            targetSdkVersion 021
        }
        two {
            minSdkVersion 19
        }
    }
}
repositories {
    mavenLocal()
    maven { url "file:///tmp/x" }
}
dependencies {
    implementation 'a:b:+'
    implementation 'a:b:1.+'
    implementation "a:b:1.2.+"
    implementation group: 'a', name: 'b', version: '1.+'
    implementation 'a:b:[1.0,2.0)'
    implementation 'a:b:latest.release'
    implementation 'a:b:1.0-SNAPSHOT'
    implementation project(':../foo')
    implementation project(':a:b')
    implementation project(path: ':..:bar')
    implementation files('C:\\foo.jar')
    implementation fileTree(dir: '/abs', include: ['*.jar'])
    implementation 'a:c:1.0'
    implementation "a:d:$ver"
    implementation "a:e:${ver}"
    implementation 'a:f:$ver'
    implementation('a:g:$ver')
    api 'a:i:@{ver}'
    kapt 'com.google.dagger:dagger-compiler:2.0'
    compileOnly 'com.google.auto.value:auto-value:1.5'
    annotationProcessor 'com.google.auto.value:auto-value:1.5'
    kapt 'androidx.room:room-compiler:2.0.0'
    implementation 'androidx.databinding:databinding-compiler:3.0.0'
    implementation 'com.google.android.gms:play-services-ads:19.0.0'
    implementation 'com.google.android.gms:play-services:+'
    implementation 'com.google.android.gms:play-services-base:12.0.1'
    implementation platform('com.google.firebase:firebase-bom:32.0.0')
    implementation 'com.google.firebase:firebase-bom:32.0.0'
    implementation('com.google.firebase:firebase-bom:32.0.0')
    api "com.google.firebase:firebase-bom:32.0.0"
    implementation 'androidx.lifecycle:lifecycle-common-java8:2.0.0'
    implementation 'androidx.lifecycle:lifecycle-compiler:2.0.0'
    testCompile 'junit:junit:4.12'
    androidTestCompile 'x:y:1'
    debugCompile 'x:y:1'
    provided 'x:z:1'
    apk 'x:z:1'
}
`,
		want: map[string][]int{
			"AnnotationProcessorOnCompilePath": {51, 54, 63},
			"BomWithoutPlatform":               {59, 60, 61},
			"GradleDeprecated":                 {64, 65, 66, 67, 68},
			"GradleDynamicVersion":             {32, 33, 34, 35, 56},
			"GradlePath":                       {42},
			"NotInterpolated":                  {47, 48},
			"StringInteger":                    {5, 6},
			"UseOfBundledGooglePlayServices":   {56},
		},
	},
	{
		name: "groovy deprecated constructs and paths",
		path: "build.gradle",
		content: `apply plugin: 'com.android.application'
android {
    compileSdkVersion "android-34"
    defaultConfig {
        minSdkVersion 14
        targetSdkVersion '34'
        versionCode "5"
        versionName 7
        packageName "x.y"
        maxSdkVersion 19
        minSdk "16"
        compileSdk '33'
    }
    buildTypes {
        debug {
            packageNameSuffix "x"
            applicationIdSuffix "dbg"
            minSdkVersion '15'
        }
    }
    productFlavors {
        flavor1 {
            versionCode '4'
            minSdkVersion 19
            targetSdkVersion "29"
            applicationIdSuffix ".f1"
        }
    }
}
dependencies {
    compile fileTree(dir: 'libs', include: ['*.jar'])
    compile files('libs/a.jar')
    compile files('libs\\a.jar')
    compile files("/abs/a.jar")
    compile files('a.jar', 'b.jar')
    compile project(':sub')
    compile project(':sub\\x')
    implementation files('c:\\foo.jar')
}
`,
		want: map[string][]int{
			"GradleDeprecated": {9, 16, 31, 32, 33, 34, 35, 36, 37},
			"GradlePath":       {17, 33, 34, 38},
			"StringInteger":    {6, 11},
		},
	},
	{
		name: "octal values",
		path: "build.gradle",
		content: `apply plugin: 'com.android.application'
android {
    compileSdkVersion 011
    defaultConfig {
        versionCode 0755
        versionName 0.5
        minSdkVersion 07
        targetSdkVersion 0x13
        applicationIdSuffix "x"
        compileSdk 012
    }
    productFlavors {
        a {
            minSdkVersion 012
        }
    }
}
`,
		want: map[string][]int{
			"AccidentalOctal": {3, 5, 6, 8, 10},
		},
	},
	{
		name: "groovy closures and coordinates",
		path: "build.gradle",
		content: `buildscript {
    repositories { jcenter() }
    dependencies {
        classpath 'com.android.tools.build:gradle:7.+'
        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:1.+")
    }
}
apply plugin: 'com.android.application'
apply plugin: 'kotlin-android'
def ver = '1.0'
android {
    compileSdkVersion 34
    defaultConfig {
        applicationId "x"   // comment with 'quote
        minSdkVersion 21; targetSdkVersion 34
        versionCode 2000000000
        versionName "1.0"
    }
    dataBinding { enabled = true }
    buildTypes {
        debug { applicationIdSuffix "dbg" }
        release { applicationIdSuffix ".rel"; minifyEnabled false }
    }
    productFlavors {
        dev { minSdkVersion 21 }
        prod { minSdkVersion(21) }
        other {
            if (project.hasProperty('x')) {
                minSdkVersion 21
            }
        }
    }
}
dependencies { implementation 'a:b:1.+' }
dependencies {
    implementation('a:b:$ver') {
        exclude group: 'x'
    }
    implementation "a:b:${ver}", {
        transitive = false
    }
    api platform('com.google.firebase:firebase-bom:1.0')
    api(platform('com.google.firebase:firebase-bom:1.0'))
    api enforcedPlatform('com.google.firebase:firebase-bom:1.0')
    implementation 'com.google.firebase:firebase-bom:+'
    debugApi 'io.grpc:grpc-bom:1.0'
    releaseImplementation "io.ktor:ktor-bom:2.0"
    kaptTest 'com.google.dagger:dagger-compiler:2.0'
    testImplementation 'com.google.dagger:dagger-compiler:2.0'
    androidTestCompile 'com.google.dagger:dagger-compiler:2.0'
    debugCompileOnly 'com.google.dagger:dagger-compiler:2.0'
    runtimeOnly 'com.google.dagger:dagger-compiler:2.0'
    implementation 'com.google.dagger:dagger-compiler'
    implementation group: 'org.projectlombok', name: 'lombok', version: '1.0'
    implementation group: "io.grpc", name: "grpc-bom", version: "1.+"
    implementation group: 'org.projectlombok', name: 'lombok'
    compileOnly 'org.projectlombok:lombok:1.18.20'
    implementation 'com.google.android.gms:play-services:5.0.0'
    implementation 'com.google.android.gms:play-services:6.5.0'
    implementation 'com.google.android.gms:play-services:6.4.9'
    implementation 'com.google.android.gms:play-services:+'
    implementation 'com.google.android.gms:play-services:12.+'
    implementation 'com.google.android.gms:play-services:5.+'
    implementation 'com.google.android.gms:play-services:$v'
    implementation 'com.google.android.gms:play-services-base:+'
    implementation "x:y:$ver"
    implementation 'x:y:' + ver
    implementation 'x:y:1.0@aar'
    implementation 'x:y:$ver@aar'
    testImplementation 'x:y:1+'
}
`,
		want: map[string][]int{
			"AnnotationProcessorOnCompilePath": {49, 50, 51, 54, 57},
			"BomWithoutPlatform":               {45, 46, 47, 55},
			"DataBindingWithoutKapt":           {19},
			"DevModeObsolete":                  {25},
			"GradleDeprecated":                 {50},
			"GradleDynamicVersion":             {4, 5, 34, 45, 55, 61, 62, 63, 65, 70},
			"GradlePath":                       {21},
			"HighAppVersionCode":               {16},
			"NotInterpolated":                  {64, 69},
			"UseOfBundledGooglePlayServices":   {59, 61, 62},
		},
	},
	{
		name: "kotlin script",
		path: "build.gradle.kts",
		content: `plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android") version "1.9.0"
    kotlin("android")
}
val ver = "1.0"
android {
    compileSdk = 34
    defaultConfig {
        applicationId = "x"
        minSdk = 21
        targetSdk = 34
        versionCode = 2000000000
        versionName = "1.0"
    }
    buildFeatures { dataBinding = true }
    buildTypes {
        getByName("debug") { applicationIdSuffix = "dbg" }
        create("rel") { applicationIdSuffix = ".rel" }
        debug { applicationIdSuffix = "dbg2" }
    }
    productFlavors {
        create("dev") { minSdk = 21 }
        val dev by creating { minSdk = 21 }
    }
}
dependencies {
    implementation("a:b:1.+")
    implementation(group = "a", name = "b", version = "1.+")
    implementation(platform("com.google.firebase:firebase-bom:1.0"))
    implementation("com.google.firebase:firebase-bom:1.0")
    api("io.grpc:grpc-bom:1.0")
    implementation(libs.compose.bom)
    implementation(platform(libs.compose.bom))
    kapt("com.google.dagger:dagger-compiler:2.0")
    implementation("com.google.dagger:dagger-compiler:2.0")
    compileOnly("com.google.dagger:dagger-compiler:2.0")
    testImplementation("com.google.dagger:dagger-compiler:2.0")
    implementation("com.google.dagger:dagger-compiler:2.0") { isTransitive = false }
    implementation("com.google.android.gms:play-services:12.0.1")
    implementation('x:y:$ver')
    implementation("x:y:$ver")
    implementation(files("libs/a.jar", "libs/b.jar"))
    implementation(files("C:\\libs\\a.jar"))
    implementation(fileTree("libs") { include("*.jar") })
    implementation(fileTree(mapOf("dir" to "/abs", "include" to listOf("*.jar"))))
    compile("a:b:1")
    testCompile("a:b:1")
    compile(project(":x"))
    "compile"("a:b:1")
}
`,
		want: map[string][]int{
			"AnnotationProcessorOnCompilePath": {36, 37, 38},
			"BomWithoutPlatform":               {31, 32},
			"DataBindingWithoutKapt":           {16},
			"GradleDeprecated":                 {47, 48, 49},
			"GradleDynamicVersion":             {28},
			"GradlePath":                       {18, 20, 44},
			"HighAppVersionCode":               {13},
			"NotInterpolated":                  {41},
			"UseOfBundledGooglePlayServices":   {40},
		},
	},
	{
		name: "sdk strings",
		path: "build.gradle",
		content: `apply plugin: 'com.android.application'
android {
    compileSdkVersion '33'
    compileSdk "34"
    defaultConfig {
        minSdkVersion "21"
        minSdk '21'
        targetSdk "34"
        compileSdkVersion '33'
        compileSdk '34'
        versionCode 1
    }
    productFlavors {
        a {
            compileSdk '34'
            targetSdkVersion '34'
        }
    }
}
`,
		want: map[string][]int{
			"StringInteger": {3, 4, 6, 7, 8},
		},
	},
}

func TestLintGradleParity(t *testing.T) {
	for _, tc := range lintGradleParityCases {
		cfg, _ := android.ParseBuildGradleContent(tc.content)
		for _, id := range lintGradleParityRules {
			t.Run(tc.name+"/"+id, func(t *testing.T) {
				var got []int
				seen := map[int]bool{}
				for _, f := range runGradleRule(findGradleRule(t, id), tc.path, tc.content, cfg) {
					if !seen[f.Line] {
						seen[f.Line] = true
						got = append(got, f.Line)
					}
				}
				sort.Ints(got)
				if fmt.Sprint(got) != fmt.Sprint(append([]int(nil), tc.want[id]...)) {
					t.Fatalf("%s lines = %v, Lint reports %v", id, got, tc.want[id])
				}
			})
		}
	}
}
