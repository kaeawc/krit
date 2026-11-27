package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class PosHolder(view: View) : RecyclerView.ViewHolder(view) {
    var pos = 0
}

class HolderFieldAdapter : RecyclerView.Adapter<PosHolder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = PosHolder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: PosHolder, position: Int) {
        holder.pos = position
    }
}

class TagAdapter : RecyclerView.Adapter<PosHolder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = PosHolder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: PosHolder, position: Int) {
        holder.itemView.tag = position
    }
}

class CopiedFieldAdapter : RecyclerView.Adapter<PosHolder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = PosHolder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: PosHolder, position: Int) {
        val p = position
        holder.pos = p
    }
}
