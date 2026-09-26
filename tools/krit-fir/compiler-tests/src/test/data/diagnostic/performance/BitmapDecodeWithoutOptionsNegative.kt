// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: every decode that passes BitmapFactory.Options, including a null
// Options argument (it selects the Options overload, and Go counts more than
// one argument), a callable reference (not a call), and other BitmapFactory
// and Bitmap calls. Neither Go nor FIR reports these.
package test

import android.content.res.Resources
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Rect
import java.io.FileDescriptor
import java.io.InputStream

fun withOptions(path: String, input: InputStream, res: Resources, id: Int, data: ByteArray, fd: FileDescriptor) {
    val options = BitmapFactory.Options().apply { inSampleSize = 2 }
    BitmapFactory.decodeFile(path, options)
    BitmapFactory.decodeStream(input, null, options)
    BitmapFactory.decodeStream(input, Rect(), options)
    BitmapFactory.decodeResource(res, id, options)
    BitmapFactory.decodeByteArray(data, 0, data.size, options)
    BitmapFactory.decodeFileDescriptor(fd, null, options)
    android.graphics.BitmapFactory.decodeFile(path, options)
}

fun nullOptions(path: String, input: InputStream) {
    BitmapFactory.decodeFile(path, null)
    BitmapFactory.decodeStream(input, null, null)
}

fun reference(): (String) -> Bitmap? = BitmapFactory::decodeFile

fun optionsOnly(): BitmapFactory.Options = BitmapFactory.Options()
