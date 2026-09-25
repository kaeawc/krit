// Smoke: package-manager queries and the PERMISSION_GRANTED comparison idiom.
package stubs

import android.content.Context
import android.content.pm.PackageInfo
import android.content.pm.PackageManager

fun hasCamera(context: Context): Boolean {
    val pm: PackageManager = context.packageManager
    val info: PackageInfo? = try {
        pm.getPackageInfo(context.packageName, PackageManager.GET_META_DATA)
    } catch (e: PackageManager.NameNotFoundException) {
        null
    }
    println(info?.versionName)
    return pm.hasSystemFeature(PackageManager.FEATURE_CAMERA) &&
        context.checkSelfPermission(android.Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
}
