package com.example

import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class BlockRedeclaresAdapter(private val flag: Boolean) : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        if (flag) {
            val position = 3
            holder.itemView.setOnClickListener(object : View.OnClickListener {
                override fun onClick(v: View) {
                    println(position)
                }
            })
        }
    }
}

class CopyRedeclaredAdapter(private val flag: Boolean) : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        var c = 0
        c = position
        if (flag) {
            val c = 5
            holder.itemView.setOnClickListener(object : View.OnClickListener {
                override fun onClick(v: View) {
                    println(c)
                }
            })
        }
    }
}

class TopLevelLocalAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        val position = holder.bindingAdapterPosition
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                println(position)
            }
        })
    }
}

class BindWhenAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        when (val position = holder.bindingAdapterPosition) {
            RecyclerView.NO_POSITION -> Unit
            else -> holder.itemView.setOnClickListener(object : View.OnClickListener {
                override fun onClick(v: View) {
                    println(position)
                }
            })
        }
    }
}

class LambdaCopyAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        holder.itemView.post {
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
