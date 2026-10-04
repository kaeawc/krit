// Smoke: the newInstance + bundleOf arguments idiom on a no-arg Fragment.
package stubs

import android.os.Bundle
import androidx.core.os.bundleOf
import androidx.fragment.app.Fragment

class BundleArgsFragment : Fragment() {
    private val userId: String?
        get() = arguments?.getString(ARG_USER_ID)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (userId == null) arguments = bundleOf()
    }

    companion object {
        private const val ARG_USER_ID = "userId"

        fun newInstance(userId: String): BundleArgsFragment = BundleArgsFragment().apply {
            arguments = bundleOf(ARG_USER_ID to userId, "retries" to 3)
        }
    }
}
