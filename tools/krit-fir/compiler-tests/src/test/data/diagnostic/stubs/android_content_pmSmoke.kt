// Smoke: package-manager queries and the PERMISSION_GRANTED comparison idiom.
package stubs

import android.content.Context
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.content.pm.SigningInfo
import android.os.Build

fun hasCamera(context: Context): Boolean {
    val pm: PackageManager = context.packageManager
    val info: PackageInfo? = try {
        pm.getPackageInfo(context.packageName, PackageManager.GET_META_DATA)
    } catch (e: PackageManager.NameNotFoundException) {
        null
    }
    <!PrintlnInProduction!>println<!>(info?.versionName)
    return pm.hasSystemFeature(PackageManager.FEATURE_CAMERA) &&
        context.checkSelfPermission(android.Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
}

// The API 28+ signing-certificate query, with the API 33 PackageInfoFlags overload.
fun signerDigests(context: Context): List<String> {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.P) return emptyList()
    val pm = context.packageManager
    val info = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
        val flags = PackageManager.PackageInfoFlags.of(PackageManager.GET_SIGNING_CERTIFICATES.toLong())
        pm.getPackageInfo(context.packageName, flags)
    } else {
        pm.getPackageInfo(context.packageName, PackageManager.GET_SIGNING_CERTIFICATES)
    }
    val signing: SigningInfo = info.signingInfo ?: return emptyList()
    val signers = if (signing.hasMultipleSigners()) signing.apkContentsSigners else signing.signingCertificateHistory
    return signers.map { it.toCharsString() }
}
