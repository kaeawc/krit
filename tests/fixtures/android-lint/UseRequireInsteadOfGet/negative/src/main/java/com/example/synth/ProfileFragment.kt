package com.example.synth

import android.content.Context
import android.os.Bundle
import android.view.View
import androidx.fragment.app.Fragment

class ProfileFragment : Fragment() {
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        val ctx = requireContext()
        // Kotlin's requireNotNull is not one of the null checks Lint rewrites.
        val args = requireNotNull(arguments)
        val host = checkNotNull(activity) { "Not attached" }
        val maybe = context?.packageName
    }

    fun local() {
        val activity: Context? = null
        activity!!.packageName
    }
}

class Holder(val context: Context?) {
    fun name() = context!!.packageName
}
