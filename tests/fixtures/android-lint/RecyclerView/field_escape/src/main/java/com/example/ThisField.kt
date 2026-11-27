package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

// `this.f` is the property even where a local f is in scope.
class ThisFieldAdapter : RecyclerView.Adapter<PosHolder>() {
    var f = 0

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = PosHolder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: PosHolder, position: Int) {
        var f = 0
        this.f = position
        f++
    }
}

class ThisFieldNoLocalAdapter : RecyclerView.Adapter<PosHolder>() {
    var f = 0

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = PosHolder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: PosHolder, position: Int) {
        this.f = position
    }
}
