package dev.jasonpearson.krit.fir.checkers.licensing

internal fun isInOptInMarkerPackageFamily(apiPackage: String, markerPackage: String): Boolean {
    val knownRoots = listOf("kotlin", "coil", "coil3", "com.google.accompanist", "com.google.android.horologist")
    val root = knownRoots.firstOrNull { markerPackage == it || markerPackage.startsWith("$it.") }
        ?: markerPackage.split('.').take(2).joinToString(".")
    return apiPackage == root || apiPackage.startsWith("$root.")
}
