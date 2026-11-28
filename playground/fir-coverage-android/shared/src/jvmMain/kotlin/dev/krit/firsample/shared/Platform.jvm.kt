package dev.krit.firsample.shared

actual fun platformName(): String = "JVM ${System.getProperty("java.version")}"
