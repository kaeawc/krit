// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 31, 49, 67, 87x2
// Adapters that call notifyDataSetChanged() with no DiffUtil or ListAdapter in the class.
package test

import android.annotation.SuppressLint
import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view)

class PlainAdapter : RecyclerView.Adapter<Holder>() {
    private var items = listOf<String>()

    fun update(newItems: List<String>) {
        items = newItems
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = items.size
}

// Only the first call is reported.
class TwoCallsAdapter : RecyclerView.Adapter<Holder>() {
    fun first() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    fun second() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// A call inside a lambda, with an explicit receiver.
class LambdaAdapter : RecyclerView.Adapter<Holder>() {
    fun load(source: List<String>) {
        source.forEach { println(it) }
        run { this.<!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>() }
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// The first call is suppressed; the next unsuppressed call is the one reported.
class SecondUnsuppressedAdapter : RecyclerView.Adapter<Holder>() {
    @SuppressLint("NotifyDataSetChanged")
    fun first() {
        notifyDataSetChanged()
    }

    fun second() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// Go reports a nested adapter's call once for it and once for the outer adapter (two findings on one line); one finding here.
class OuterAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0

    class InnerAdapter : RecyclerView.Adapter<Holder>() {
        fun refresh() {
            <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
        }

        override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

        override fun onBindViewHolder(holder: Holder, position: Int) = Unit

        override fun getItemCount(): Int = 0
    }
}
