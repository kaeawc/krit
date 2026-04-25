package com.example

import android.view.LayoutInflater
import android.view.ViewGroup

class MyAdapter {
    fun noRootParams(inflater: LayoutInflater) {
        val view = inflater.inflate(R.layout.no_root_layout_params, null)
    }

    fun createView(inflater: LayoutInflater, parent: ViewGroup) {
        val view = inflater.inflate(R.layout.with_root_layout_params, parent, false)
    }

    fun dialog(inflater: LayoutInflater) {
        val view = inflater.inflate(R.layout.with_root_layout_params, null)
        AlertDialog.Builder(context).setView(view)
    }

    fun compose() {
        AndroidView(
            factory = {
                LayoutInflater.from(it).inflate(R.layout.with_root_layout_params, null)
            }
        )
    }
}
