package test

import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

class FilteringInputStream(input: InputStream) : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> {
        return super.resolveClass(desc)
    }
}

class JsonDecoder {
    fun decode(text: String): String {
        return kotlinx.serialization.json.Json.decodeFromString(text)
    }
}
