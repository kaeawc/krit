package test

import okhttp3.CertificatePinner

class Builder {
    fun build() = Any()
}

fun pinned() {
    CertificatePinner.Builder()
        .add("example.com", "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
        .build()
    Builder().build()
}
