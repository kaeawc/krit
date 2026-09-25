// Smoke: SAM-converted window-insets listener and the isVisible/isGone extensions.
package stubs

import android.view.View
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.isGone
import androidx.core.view.isVisible

fun insets(view: View) {
    ViewCompat.setOnApplyWindowInsetsListener(view) { v, insets ->
        v.isVisible = insets.isVisible(WindowInsetsCompat.Type.ime())
        insets
    }
    view.isGone = !view.isVisible
    ViewCompat.setElevation(view, 4f)
}
