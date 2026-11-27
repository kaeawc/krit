package com.example.remoteviewlayout

import android.content.Context
import android.widget.RemoteViews

fun views(context: Context) = RemoteViews(context.packageName, R.layout.widget)
