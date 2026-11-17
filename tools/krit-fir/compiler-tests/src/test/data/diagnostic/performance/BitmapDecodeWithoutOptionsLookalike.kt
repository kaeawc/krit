// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: declarations of the user's own that are only named
// BitmapFactory. Neither decodes an Android bitmap, so the message
// ("BitmapFactory.decodeFile without BitmapFactory.Options may decode a
// full-size bitmap") would be false of this code. Go skips a receiver the
// file declares or does not import from android.graphics, and a qualifier
// other than android.graphics. Other receivers with decode methods are
// reported by neither.
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
