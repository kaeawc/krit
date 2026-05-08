// Conflicts: pins okhttp to 4.11.0 and gson to 2.9.0, both of which differ
// from gradle/libs.versions.toml. retrofit matches the catalog and must NOT
// trigger a finding.
object Deps {
    const val OKHTTP = "com.squareup.okhttp3:okhttp:4.11.0"
    const val GSON = "com.google.code.gson:gson:2.9.0"
    const val RETROFIT = "com.squareup.retrofit2:retrofit:2.10.0"
}
