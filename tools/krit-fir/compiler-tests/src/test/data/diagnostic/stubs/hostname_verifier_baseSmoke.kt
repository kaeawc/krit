package stubs

import fixture.hostname.VerifierBase
import javax.net.ssl.SSLSession

class StrictVerifier : VerifierBase() {
    override fun verify(hostname: String?, session: SSLSession?): Boolean = false
}
