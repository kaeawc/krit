// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 65, 82
// Shapes where the compiler's view differs from Go's name matching.
package test

import android.view.View
import android.view.ViewGroup
import android.widget.BaseAdapter
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view)

object RowDiff : DiffUtil.ItemCallback<String>() {
    override fun areItemsTheSame(oldItem: String, newItem: String): Boolean = oldItem == newItem

    override fun areContentsTheSame(oldItem: String, newItem: String): Boolean = oldItem == newItem
}

abstract class BaseRows : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// Go needs a direct supertype named like an adapter; BaseRows is one through its base class.
class Rows : BaseRows() {
    fun refresh() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }
}

// Go visits class declarations only; an object adapter has the same problem.
object SingletonAdapter : RecyclerView.Adapter<Holder>() {
    fun refresh() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// Go reads the name of the direct supertype only; this adapter is a ListAdapter through its base class.
abstract class BaseList : ListAdapter<String, Holder>(RowDiff)

class DerivedList : BaseList() {
    fun refresh() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit
}

// Go matches the call by name; the call here is a ListView adapter's, not the RecyclerView adapter's.
class HostAdapter(private val legacy: BaseAdapter) : RecyclerView.Adapter<Holder>() {
    fun refresh() {
        legacy.notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

class Observer {
    fun notifyDataSetChanged() = Unit
}

// A same-named function on an unrelated class.
class ObservingAdapter(private val observer: Observer) : RecyclerView.Adapter<Holder>() {
    fun refresh() {
        observer.notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}
