// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 19, 22x2, 24
// Positives: an Options-less decode as an operand, an argument, a branch, or
// a returned value.
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory

fun elvis(cached: Bitmap?, path: String): Bitmap? = cached ?: <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(path)<!>

fun branch(fresh: Boolean, path: String): Bitmap? = if (fresh) <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(path)<!> else null

fun argument(path: String): List<Bitmap?> = listOf(<!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(path)<!>)

fun comparison(cached: Bitmap?, path: String): Boolean = cached == <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(path)<!>

fun returned(path: String): Bitmap? {
    return <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(path)<!>
}

fun pair(first: String, second: String): List<Bitmap?> = listOf(<!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(first)<!>, <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(second)<!>)

fun safeChain(path: String): Int? = <!BitmapDecodeWithoutOptions!>BitmapFactory.decodeFile(path)<!>?.height
