// Smoke: Fragment lifecycle overrides, view-lifecycle coroutines, dialogs, transactions.
package stubs

import android.app.Dialog
import android.content.Context
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment
import androidx.fragment.app.FragmentActivity
import androidx.fragment.app.FragmentManager
import androidx.fragment.app.activityViewModels
import androidx.fragment.app.viewModels
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ViewModel
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.launch

class SmokeFragmentViewModel : ViewModel()

class SmokeFragment : Fragment() {
    private val viewModel: SmokeFragmentViewModel by viewModels()
    private val shared: SmokeFragmentViewModel by activityViewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }

    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View? =
        inflater.inflate(android.R.layout.simple_list_item_1, container, false)

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                println(view.id)
            }
        }
        val ctx: Context = requireContext()
        val host: FragmentActivity = requireActivity()
        val args: Bundle? = arguments
        println("$viewModel $shared $ctx $host $args $context ${getString(android.R.string.ok)}")
    }

    override fun onAttach(context: Context) {
        super.onAttach(context)
    }

    override fun onDestroyView() {
        super.onDestroyView()
    }
}

class SmokeDialogFragment : DialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog = Dialog(requireContext())
}

fun showDialog(activity: FragmentActivity) {
    val manager: FragmentManager = activity.supportFragmentManager
    SmokeDialogFragment().show(manager, "dialog")
    manager.beginTransaction()
        .replace(android.R.id.content, SmokeFragment(), "tag")
        .addToBackStack(null)
        .commit()
    val owner: LifecycleOwner = SmokeFragment()
    println(manager.findFragmentByTag("tag") ?: owner)
}
