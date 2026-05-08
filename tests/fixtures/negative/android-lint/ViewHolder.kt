package test

import androidx.recyclerview.widget.RecyclerView

class MyAdapter : RecyclerView.Adapter<MyAdapter.MyViewHolder>() {
    class MyViewHolder(view: android.view.View) : RecyclerView.ViewHolder(view)

    override fun onCreateViewHolder(parent: android.view.ViewGroup, viewType: Int): MyViewHolder {
        val view = android.view.LayoutInflater.from(parent.context).inflate(R.layout.item, parent, false)
        return MyViewHolder(view)
    }

    override fun onBindViewHolder(holder: MyViewHolder, position: Int) {}
    override fun getItemCount(): Int = 0
}
