package test

import okhttp3.CertificatePinner

fun pinned() {
    CertificatePinner.Builder()
        .add("example.com", "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
        .build()
}
