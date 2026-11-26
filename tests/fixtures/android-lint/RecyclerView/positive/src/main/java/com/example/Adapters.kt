package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view)

class FieldAdapter : RecyclerView.Adapter<Holder>() {
    private var lastPosition = 0

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        lastPosition = position
    }
}

class AnonymousAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                println(position)
            }
        })
    }
}

class LambdaAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener { println(position) }
    }
}
