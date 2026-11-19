// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 24
// android.graphics.BitmapFactory decodes spelled without the `BitmapFactory`
// receiver. Each is an Options-less decode, so the message is true of it. Go
// resolves the import alias and the statically imported methods through the
// file's imports and reports them too.
// Divergence (recall): Go does not expand type aliases, so it misses Decoder.
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory as Factory
import android.graphics.BitmapFactory.decodeFile
import android.graphics.BitmapFactory.decodeStream as readStream
import java.io.InputStream

typealias Decoder = android.graphics.BitmapFactory

fun importAlias(path: String): Bitmap? = Factory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun typeAlias(path: String): Bitmap? = Decoder.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun staticImport(path: String): Bitmap? = <!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun staticImportAlias(input: InputStream): Bitmap? = <!BitmapDecodeWithoutOptions!>readStream<!>(input)

fun withOptions(path: String): Bitmap? = decodeFile(path, Factory.Options())
