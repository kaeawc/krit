// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): a Kotlin subclass of android.graphics.BitmapFactory
// calling the inherited static decodeFile unqualified. The call resolves to
// BitmapFactory.decodeFile(String), which takes no Options, so the message is
// true of it; Go misses it because the call has no navigation receiver.
package test

import android.graphics.Bitmap
import android.graphics.BitmapFactory

class MyFactory : BitmapFactory() {
    fun load(path: String): Bitmap? = <!BitmapDecodeWithoutOptions!>decodeFile(path)<!>

    fun loadWithOptions(path: String): Bitmap? = decodeFile(path, Options())
}
