// All buildSrc coordinates either match the catalog (okhttp, gson) or refer
// to an artifact that is not declared in the catalog (kotlinx-coroutines).
// Coordinates that look like dependency strings inside comments and KDoc
// must also be ignored.
//
// Comment example: "com.squareup.okhttp3:okhttp:4.11.0" must not trigger.
/**
 * KDoc example: "com.google.code.gson:gson:2.9.0" must not trigger.
 */
object Deps {
    const val OKHTTP = "com.squareup.okhttp3:okhttp:4.12.0"
    const val GSON = "com.google.code.gson:gson:2.10.1"
    const val COROUTINES = "org.jetbrains.kotlinx:kotlinx-coroutines-core:1.7.3"
}
