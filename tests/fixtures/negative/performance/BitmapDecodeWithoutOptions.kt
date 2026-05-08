package fixtures.negative.performance

import android.graphics.BitmapFactory
import java.io.InputStream

fun loadBitmap(path: String, stream: InputStream, resId: Int) {
    val fileOptions = BitmapFactory.Options().apply { inSampleSize = 2 }
    BitmapFactory.decodeFile(path, fileOptions)

    val resourceOptions = BitmapFactory.Options()
    BitmapFactory.decodeResource(null, resId, resourceOptions)

    val streamOptions = BitmapFactory.Options()
    BitmapFactory.decodeStream(stream, null, streamOptions)
}
