// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 29, 30
// Divergence (precision): declarations of the user's own that are only named
// BitmapFactory. Go matches the receiver by the last segment of its text, so
// it reports `BitmapFactory.decodeFile(path)` and
// `Codecs.BitmapFactory.decodeStream(path)` here, but neither decodes an
// Android bitmap: the message ("BitmapFactory.decodeFile without
// BitmapFactory.Options may decode a full-size bitmap") is false of this
// code. Other receivers with decode methods are reported by neither.
package test

object BitmapFactory {
    fun decodeFile(path: String): String = path
}

object Codecs {
    object BitmapFactory {
        fun decodeStream(path: String): String = path
    }
}

class ImageDecoder {
    fun decodeFile(path: String): String = path
}

fun decodeFile(path: String): String = path

fun lookalikes(path: String, decoder: ImageDecoder) {
    BitmapFactory.decodeFile(path)
    Codecs.BitmapFactory.decodeStream(path)
    decoder.decodeFile(path)
    decodeFile(path)
}
