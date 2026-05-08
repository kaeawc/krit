package test

import java.io.File

fun save(dir: File, json: String) {
    File(dir, "cache.json").writeText(json)
}
