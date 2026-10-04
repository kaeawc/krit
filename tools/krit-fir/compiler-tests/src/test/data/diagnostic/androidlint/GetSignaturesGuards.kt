// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 87, 102, 108, 111, 115, 126, 131, 136, 141
// The API guards Go honors, matched by spelling as Go matches them: an
// enclosing function, class, object, or property annotated @RequiresApi or
// @TargetApi, and an enclosing if or when that reads Build.VERSION.SDK_INT
// anywhere, at any distance from the call.
package test

import android.annotation.TargetApi
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.os.Build
import androidx.annotation.RequiresApi

fun ifGuard(pm: PackageManager): PackageInfo =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    } else {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }

fun whenGuard(pm: PackageManager): PackageInfo = when {
    Build.VERSION.SDK_INT >= 28 -> pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    else -> pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
}

fun whenSubjectGuard(pm: PackageManager): PackageInfo? = when (Build.VERSION.SDK_INT) {
    27 -> pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    else -> null
}

// The SDK_INT read may sit in the branch itself.
fun branchGuard(pm: PackageManager, legacy: Boolean) {
    if (legacy) {
        println(Build.VERSION.SDK_INT)
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }
}

fun lambdaInsideGuard(pm: PackageManager, names: List<String>) {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.P) {
        names.forEach { pm.getPackageInfo(it, PackageManager.GET_SIGNATURES) }
    }
}

fun localFunctionInsideGuard(pm: PackageManager) {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.P) {
        fun legacy() = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
        legacy()
    }
}

@RequiresApi(27)
fun requiresApi(pm: PackageManager) = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

@TargetApi(27)
fun targetApi(pm: PackageManager) = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

@androidx.annotation.RequiresApi(27)
fun qualifiedAnnotation(pm: PackageManager) = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

@RequiresApi(27)
class AnnotatedClass(private val pm: PackageManager) {
    fun member() = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

    class Nested(private val pm: PackageManager) {
        fun member() = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }
}

@RequiresApi(27)
object AnnotatedObject {
    fun member(pm: PackageManager) = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
}

class AnnotatedProperties(private val pm: PackageManager) {
    @RequiresApi(27)
    val member: PackageInfo = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

    @get:RequiresApi(27)
    val inlineGetter: PackageInfo get() = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

    // Go's tree nests a getter in the property only when it starts on the
    // header's line; a getter on its own line is outside the annotation.
    @get:RequiresApi(27)
    val ownLineGetter: PackageInfo
        get() = <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>

    @get:RequiresApi(27)
    val viaGetterInitializer: PackageInfo = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)

    fun local() {
        @TargetApi(27)
        val info = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
        println(info)
    }
}

// Not guards, for Go and FIR alike.
fun earlyReturn(pm: PackageManager): PackageInfo? {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) return null
    return <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>
}

// Go matches the spelling `Build.VERSION.SDK_INT`, so a fully qualified read
// does not guard the call.
fun qualifiedSdkInt(pm: PackageManager): PackageInfo? =
    if (android.os.Build.VERSION.SDK_INT < 28) <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!> else null

fun unrelatedIf(pm: PackageManager, legacy: Boolean): PackageInfo? =
    if (legacy) <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!> else null

// An SDK_INT check inside the call's own argument is not an enclosing guard.
fun guardInsideArgument(pm: PackageManager): PackageInfo =
    <!GetSignatures!>pm<!>.getPackageInfo(
        "com.example",
        if (Build.VERSION.SDK_INT < 28) PackageManager.GET_SIGNATURES else PackageManager.GET_SIGNING_CERTIFICATES,
    )

class Holder(private val pm: PackageManager) {
    // Go reads the annotations of function, class, object, and property
    // declarations only: a companion object, a constructor, and an accessor
    // are other nodes.
    @RequiresApi(27)
    companion object {
        fun companionMember(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>
    }

    @RequiresApi(27)
    constructor(pm: PackageManager, eager: Boolean) : this(pm) {
        if (eager) println(<!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>)
    }

    val getter: PackageInfo
        @RequiresApi(27)
        get() = <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>

    // Another annotation on the property is not a guard.
    fun otherAnnotation() {
        @Suppress("UNUSED_VARIABLE")
        val info = <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>
    }
}
