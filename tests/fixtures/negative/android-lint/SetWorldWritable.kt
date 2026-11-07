package com.example

import java.io.File

class Exporter {
    fun share(file: File) {
        file.setWritable(true, true)
    }
}
