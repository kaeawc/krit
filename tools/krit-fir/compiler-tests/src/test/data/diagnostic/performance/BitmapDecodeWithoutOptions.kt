// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 14, 16, 18, 21, 24, 26, 29, 31, 34, 39, 43, 47
// Positives: BitmapFactory.decodeFile(path) and decodeStream(stream), the
// Options-less overloads Go reports, however the call is placed. Go reports
// each of these too, on the call's first line.
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import java.io.InputStream

fun file(path: String): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun stream(input: InputStream): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeStream<!>(input)

fun qualified(path: String): Bitmap? = android.graphics.BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun chained(path: String): Int = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path).width

fun statement(path: String) {
    BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)
}

fun inLambda(paths: List<String>): List<Bitmap?> = paths.map { BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(it) }

fun nullableArgument(path: String?): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

class Loader(private val input: InputStream) {
    val eager: Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeStream<!>(input)

    fun load(path: String): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

    companion object {
        fun load(path: String): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)
    }
}

object Singleton {
    fun load(path: String): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)
}

val anonymous = object {
    fun load(path: String): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)
}

fun local(path: String): Bitmap? {
    fun inner(): Bitmap? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)
    return inner()
}
