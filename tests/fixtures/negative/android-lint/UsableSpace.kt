package com.example

import java.io.File

class Disk {
    fun free(dir: File): Long = dir.freeSpace
}
