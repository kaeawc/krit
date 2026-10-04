package com.example

import android.os.Bundle
import android.view.LayoutInflater
import android.view.ViewGroup
import androidx.core.os.bundleOf
import androidx.fragment.app.Fragment

class MyFragment : Fragment() {
    // No-arg constructor only, this is fine
}

class LoginFragment : Fragment() {
    private var userId: String? = null

    // Arguments passed via Bundle, not constructor
    companion object {
        fun newInstance(userId: String) = LoginFragment().apply {
            arguments = bundleOf("userId" to userId)
        }
    }

    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ) = inflater.inflate(android.R.layout.simple_list_item_1, container, false)
}
