package test

import okhttp3.CertificatePinner

fun emptyPinner() {
    CertificatePinner.Builder().build()
}
