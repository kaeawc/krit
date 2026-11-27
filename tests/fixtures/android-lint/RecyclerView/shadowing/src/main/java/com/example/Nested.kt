package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class NestedCopyAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        if (position > 0) {
            var p = 0
            p = position
            holder.itemView.setOnClickListener(object : View.OnClickListener {
                override fun onClick(v: View) {
                    println(p)
                }
            })
        }
    }
}

class DestructuredAdapter(private val pair: Pair<Int, Int>) : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                val (position, other) = pair
                println(position + other)
            }
        })
    }
}

class WhenSubjectAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                when (val position = holder.bindingAdapterPosition) {
                    RecyclerView.NO_POSITION -> Unit
                    else -> println(position)
                }
            }
        })
    }
}
