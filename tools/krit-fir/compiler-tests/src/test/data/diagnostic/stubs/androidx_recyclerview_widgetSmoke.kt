// Smoke: ListAdapter with a DiffUtil.ItemCallback, a plain Adapter, and wiring.
package stubs

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView

data class ListRow(val id: Long, val title: String)

class RowViewHolder(view: View) : RecyclerView.ViewHolder(view) {
    fun bind(row: ListRow) {
        (itemView as TextView).text = row.title
        println(bindingAdapterPosition)
    }
}

object RowDiff : DiffUtil.ItemCallback<ListRow>() {
    override fun areItemsTheSame(oldItem: ListRow, newItem: ListRow): Boolean = oldItem.id == newItem.id

    override fun areContentsTheSame(oldItem: ListRow, newItem: ListRow): Boolean = oldItem == newItem
}

class RowAdapter : ListAdapter<ListRow, RowViewHolder>(RowDiff) {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): RowViewHolder =
        RowViewHolder(LayoutInflater.from(parent.context).inflate(android.R.layout.simple_list_item_1, parent, false))

    override fun onBindViewHolder(holder: RowViewHolder, position: Int) {
        holder.bind(getItem(position))
    }
}

class PlainAdapter(private val rows: List<ListRow>) : RecyclerView.Adapter<RowViewHolder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): RowViewHolder =
        RowViewHolder(TextView(parent.context))

    override fun onBindViewHolder(holder: RowViewHolder, position: Int) {
        holder.bind(rows[position])
    }

    override fun getItemCount(): Int = rows.size

    fun refresh() {
        notifyDataSetChanged()
        notifyItemChanged(0)
        notifyItemInserted(1)
        notifyItemRemoved(2)
    }
}

fun attach(recyclerView: RecyclerView, rows: List<ListRow>) {
    recyclerView.layoutManager = LinearLayoutManager(recyclerView.context)
    val adapter = RowAdapter()
    recyclerView.adapter = adapter
    recyclerView.setHasFixedSize(true)
    adapter.submitList(rows)
    println(adapter.currentList.size + adapter.getItemCount())
}
