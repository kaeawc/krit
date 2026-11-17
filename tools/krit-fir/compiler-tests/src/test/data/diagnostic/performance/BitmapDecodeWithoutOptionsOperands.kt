// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 19, 22x2, 24
// Positives: an Options-less decode as an operand, an argument, a branch, or
// a returned value.
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory

fun elvis(cached: Bitmap?, path: String): Bitmap? = cached ?: BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun branch(fresh: Boolean, path: String): Bitmap? = if (fresh) BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path) else null

fun argument(path: String): List<Bitmap?> = listOf(BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path))

fun comparison(cached: Bitmap?, path: String): Boolean = cached == BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)

fun returned(path: String): Bitmap? {
    return BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)
}

fun pair(first: String, second: String): List<Bitmap?> = listOf(BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(first), BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(second))

fun safeChain(path: String): Int? = BitmapFactory.<!BitmapDecodeWithoutOptions!>decodeFile<!>(path)?.height
