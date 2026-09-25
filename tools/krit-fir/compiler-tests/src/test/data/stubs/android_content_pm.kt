// Compiler-test source stubs; never packaged in the production artifact.
package android.content.pm

import android.util.AndroidException

// Java abstract class app code only receives from Context.getPackageManager();
// modeled as an object so `PackageManager.PERMISSION_GRANTED` keeps its Java
// callable id.
object PackageManager {
    const val PERMISSION_GRANTED: Int = 0
    const val PERMISSION_DENIED: Int = -1
    const val GET_META_DATA: Int = 128
    const val FEATURE_CAMERA: String = "android.hardware.camera"

    fun getPackageInfo(packageName: String, flags: Int): PackageInfo = TODO()

    fun checkPermission(permName: String, packageName: String): Int = TODO()

    fun hasSystemFeature(featureName: String): Boolean = TODO()

    fun getLaunchIntentForPackage(packageName: String): android.content.Intent? = TODO()

    class NameNotFoundException : AndroidException {
        constructor()

        constructor(name: String?)
    }
}

open class PackageInfo {
    @JvmField
    var packageName: String? = null

    @JvmField
    var versionName: String? = null

    val longVersionCode: Long
        get() = TODO()
}
