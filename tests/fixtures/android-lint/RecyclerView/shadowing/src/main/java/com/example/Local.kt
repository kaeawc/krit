package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class LocalAdapter(private val listener: (Int) -> Unit) : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                val position = holder.bindingAdapterPosition
                if (position != RecyclerView.NO_POSITION) listener(position)
            }
        })
    }
}

class MemberAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            var position = 0

            override fun onClick(v: View) {
                println(position)
            }
        })
    }
}

class LaterLocalAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                println(position)
                val position = 3
                println(position)
            }
        })
    }
}
