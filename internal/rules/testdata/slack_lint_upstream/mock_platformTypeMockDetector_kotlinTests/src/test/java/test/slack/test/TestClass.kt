package slack.test

import slack.test.mockito.mock
import java.lang.Runnable
import kotlin.io.FileTreeWalk
import android.graphics.Typeface
import androidx.collection.ArrayMap

class MyTests {
  fun example() {
    // java.
    mock<Comparable<String>>()
    mock<Runnable>()
    // kotlin.
    mock<FileTreeWalk>()
    mock<Lazy<String>>()
    // android.
    mock<Typeface>()
    // androidx.
    mock<ArrayMap<String, String>>()
  }
}
