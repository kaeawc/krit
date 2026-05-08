package test

import android.content.Context
import android.widget.Toast

fun showDebugToast(context: Context) {
    Toast.makeText(context, "DEBUG: user clicked", Toast.LENGTH_SHORT).show()
}
