package test

import android.widget.BaseAdapter

class MyAdapter : BaseAdapter() {
    override fun getCount(): Int = 0
    override fun getItem(position: Int): Any? = null
    override fun getItemId(position: Int): Long = 0L
    override fun getView(position: Int, convertView: android.view.View?, parent: android.view.ViewGroup?): android.view.View {
        val view = layoutInflater.inflate(R.layout.item, parent, false)
        return view
    }
}
