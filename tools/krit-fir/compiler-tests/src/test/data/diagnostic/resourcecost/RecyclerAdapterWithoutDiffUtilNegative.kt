// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Adapters Go and FIR both leave alone.
package test

import android.annotation.SuppressLint
import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view)

object RowDiff : DiffUtil.ItemCallback<String>() {
    override fun areItemsTheSame(oldItem: String, newItem: String): Boolean = oldItem == newItem

    override fun areContentsTheSame(oldItem: String, newItem: String): Boolean = oldItem == newItem
}

// A ListAdapter already diffs.
class ListBacked : ListAdapter<String, Holder>(RowDiff) {
    fun refresh() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit
}

// DiffUtil in the class: a call on it.
class DiffingAdapter : RecyclerView.Adapter<Holder>() {
    private var items = listOf<String>()

    fun update(newItems: List<String>) {
        val callback = object : DiffUtil.Callback() {
            override fun getOldListSize(): Int = items.size

            override fun getNewListSize(): Int = newItems.size

            override fun areItemsTheSame(oldItemPosition: Int, newItemPosition: Int): Boolean = true

            override fun areContentsTheSame(oldItemPosition: Int, newItemPosition: Int): Boolean = true
        }
        items = newItems
        DiffUtil.calculateDiff(callback)
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = items.size
}

// DiffUtil in a constructor parameter type.
class CallbackAdapter(private val callback: DiffUtil.ItemCallback<String>) : RecyclerView.Adapter<Holder>() {
    fun refresh() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// DiffUtil as a type argument.
class GenericAdapter : RecyclerView.Adapter<Holder>() {
    private val callbacks = mutableListOf<DiffUtil.Callback>()

    fun refresh() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = callbacks.size
}

// The lint id is suppressed on the call's function and on the class.
class SuppressedOnFunction : RecyclerView.Adapter<Holder>() {
    @SuppressLint("NotifyDataSetChanged")
    fun refresh() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

@SuppressLint("NotifyDataSetChanged")
class SuppressedOnClass : RecyclerView.Adapter<Holder>() {
    fun refresh() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// No notifyDataSetChanged call: granular notifications are fine.
class GranularAdapter : RecyclerView.Adapter<Holder>() {
    fun refresh() {
        notifyItemChanged(0)
        notifyItemInserted(1)
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// Not an adapter at all.
class Notifier {
    fun notifyDataSetChanged() = Unit

    fun refresh() {
        notifyDataSetChanged()
    }
}
