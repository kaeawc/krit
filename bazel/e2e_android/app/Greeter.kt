package e2e.android

import android.content.Context

// Context comes from android.jar (the Android SDK toolchain) and R from the
// target's R class jar; with either missing the file does not compile.
fun title(context: Context): String = context.getString(R.string.title)

// The FIR-only finding: R.string.title is a non-null Int.
fun titleId(): Int? = R.string.title?.plus(0)
