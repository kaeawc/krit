package com.example

class ProguardConfig {
    val rules = """
        -dontobfuscate
        -dontwarn kotlin.**
    """
}
