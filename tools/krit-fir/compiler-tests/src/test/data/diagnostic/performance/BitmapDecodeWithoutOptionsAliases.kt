// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): android.graphics.BitmapFactory decodes spelled without
// the `BitmapFactory` receiver text Go matches. Each is the Options-less
// decodeFile, so the message is true of it; Go misses the import alias, the
// typealias, and the statically imported method (no navigation receiver).
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory as Factory
import android.graphics.BitmapFactory.decodeFile
import android.graphics.BitmapFactory.decodeStream as readStream
import java.io.InputStream

typealias Decoder = android.graphics.BitmapFactory

fun importAlias(path: String): Bitmap? = <!BitmapDecodeWithoutOptions!>Factory.decodeFile(path)<!>

fun typeAlias(path: String): Bitmap? = <!BitmapDecodeWithoutOptions!>Decoder.decodeFile(path)<!>

fun staticImport(path: String): Bitmap? = <!BitmapDecodeWithoutOptions!>decodeFile(path)<!>

fun staticImportAlias(input: InputStream): Bitmap? = <!BitmapDecodeWithoutOptions!>readStream(input)<!>

fun withOptions(path: String): Bitmap? = decodeFile(path, Factory.Options())
