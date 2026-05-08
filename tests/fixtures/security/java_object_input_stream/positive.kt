package test

import java.io.FileInputStream
import java.io.ObjectInputStream

class Decoder {
    fun decode(path: String): Any {
        return ObjectInputStream(FileInputStream(path)).use { it.readObject() }
    }
}
