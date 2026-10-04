// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): true positives Go misses. Each call decodes without
// BitmapFactory.Options, so the message ("... without BitmapFactory.Options
// may decode a full-size bitmap") is true of it.
// - decodeResource(res, id): Go lists decodeResource but requires exactly one
//   argument, and the Options-less overload takes two, so Go never reports it.
// - decodeByteArray(data, offset, length) and decodeFileDescriptor(fd): Go's
//   method list stops at decodeFile, decodeResource, and decodeStream.
package test

import android.content.res.Resources
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import java.io.FileDescriptor

fun resource(res: Resources, id: Int): Bitmap? = <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeResource(res, id)<!>

fun byteArray(data: ByteArray): Bitmap? = <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeByteArray(data, 0, data.size)<!>

fun fileDescriptor(fd: FileDescriptor): Bitmap? = <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFileDescriptor(fd)<!>
