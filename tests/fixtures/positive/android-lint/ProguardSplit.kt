package com.example

class ProguardConfig {
    val rules = """
        -dontobfuscate
        -keep class com.example.MyClass { *; }
        -keepattributes Signature
    """
}
