package sdk

import android.os.Build

fun label(): String = if (Build.VERSION.SDK_INT >= 21) "lollipop" else "older"
