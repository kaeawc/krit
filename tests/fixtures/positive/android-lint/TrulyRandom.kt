package test

import java.security.SecureRandom

fun createRandom(): SecureRandom {
    return SecureRandom(byteArrayOf(1, 2, 3))
}
