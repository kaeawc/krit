// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Small images: a decode inside a function or class named for a thumbnail,
// tiny image, icon, or avatar, and decodeByteArray of a header joined to a tail
// (directly or through a local), are not reported. A plain decodeByteArray is.
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import java.io.InputStream

private val HEADER = byteArrayOf(1, 2, 3)

fun loadAvatar(path: String): Bitmap? = BitmapFactory.decodeFile(path)

class IconLoader {
    fun load(input: InputStream): Bitmap? = BitmapFactory.decodeStream(input)
}

fun joined(tail: ByteArray): Bitmap? {
    val bytes = HEADER + tail
    return BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
}

fun joinedInline(tail: ByteArray): Bitmap? = BitmapFactory.decodeByteArray(HEADER + tail, 0, tail.size)

fun plain(data: ByteArray): Bitmap? = <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeByteArray(data, 0, data.size)<!>
