package com.example.remoteviewlayout

import android.content.Context
import android.widget.RemoteViews

fun views(context: Context) = RemoteViews(context.packageName, R.layout.widget)

fun inflated(context: Context) = android.view.LayoutInflater.from(context).inflate(R.layout.other, null)

class RemoteViewsLookalike(pkg: String, layout: Int)

fun lookalike() = RemoteViewsLookalike("p", R.layout.other)
