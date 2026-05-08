package test

import java.io.File

fun makeReadable() {
    val t = File.createTempFile("secret", ".txt")
    t.setReadable(true, false)
}

fun makeWritable() {
    val t = File.createTempFile("secret", ".txt")
    t.setWritable(true, false)
}

fun makeExecutable() {
    val t = File.createTempFile("secret", ".sh")
    t.setExecutable(true, false)
}

fun viaFiles() {
    val t = java.nio.file.Files.createTempFile("secret", ".txt").toFile()
    t.setReadable(true, false)
}
