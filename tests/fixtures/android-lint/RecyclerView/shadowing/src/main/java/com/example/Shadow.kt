package com.example

import android.view.View
import android.view.ViewGroup
import android.widget.AdapterView
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view) {
    val spinner = AdapterView::class
}

class ShadowAdapter(private val picker: AdapterView<*>) : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        picker.onItemSelectedListener = object : AdapterView.OnItemSelectedListener {
            override fun onItemSelected(p: AdapterView<*>?, v: View?, position: Int, id: Long) {
                println(position)
            }

            override fun onNothingSelected(p: AdapterView<*>?) {}
        }
    }
}

class CopyAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int) = Holder(View(parent.context))
    override fun getItemCount() = 10

    override fun onBindViewHolder(holder: Holder, position: Int) {
        val p = position
        holder.itemView.setOnClickListener(object : View.OnClickListener {
            override fun onClick(v: View) {
                println(p)
            }
        })
    }
}
