// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 18, 20
// Positives: the Options-less overloads with more or fewer arguments than
// decodeFile(path). Each call decodes without BitmapFactory.Options, so the
// message ("... without BitmapFactory.Options may decode a full-size bitmap")
// is true of it. Go counts each overload's own arguments and reports them too:
// decodeResource(res, id), decodeByteArray(data, offset, length) and
// decodeFileDescriptor(fd).
package test

import android.content.res.Resources
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import java.io.FileDescriptor

fun resource(res: Resources, id: Int): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeResource<!>(res, id)

fun byteArray(data: ByteArray): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeByteArray<!>(data, 0, data.size)

fun fileDescriptor(fd: FileDescriptor): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFileDescriptor<!>(fd)
