package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view)

class GoodAdapter(private val items: List<String>) : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = items.size

    override fun onBindViewHolder(holder: Holder, position: Int) {
        val item = items[position]
        val copy = position
        println(item + copy)
        holder.itemView.setOnClickListener { println(holder.bindingAdapterPosition) }
    }
}

class NotAnAdapter {
    private var last = 0

    fun onBindViewHolder(holder: Holder, position: Int) {
        last = position
    }
}
