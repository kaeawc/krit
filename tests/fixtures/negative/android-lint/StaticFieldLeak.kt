package com.example

import android.app.Application
import java.lang.ref.WeakReference
import android.content.Context

object Session {
    var application: Application? = null
    var weak: WeakReference<Context>? = null
}
