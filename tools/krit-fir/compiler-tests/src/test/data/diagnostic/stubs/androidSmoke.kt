// Smoke: android.R / android.Manifest constants resolve as compile-time constants.
package stubs

import android.Manifest
import android.R

const val OK_LABEL: Int = R.string.ok
const val CAMERA_PERMISSION: String = Manifest.permission.CAMERA

fun androidResources(): List<Any> = listOf(
    OK_LABEL,
    R.string.cancel,
    R.id.content,
    R.layout.simple_list_item_1,
    R.drawable.ic_menu_add,
    R.color.black,
    CAMERA_PERMISSION,
    Manifest.permission.INTERNET,
    Manifest.permission.POST_NOTIFICATIONS,
    Manifest.permission.ACCESS_FINE_LOCATION,
)
