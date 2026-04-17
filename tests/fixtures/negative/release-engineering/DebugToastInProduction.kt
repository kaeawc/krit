package test

import android.content.Context
import android.widget.Toast

fun showSavedToast(context: Context, message: String) {
    Toast.makeText(context, message, Toast.LENGTH_SHORT).show()
}
