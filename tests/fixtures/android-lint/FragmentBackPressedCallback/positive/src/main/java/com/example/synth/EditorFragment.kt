package com.example.synth

import android.os.Bundle
import android.view.View
import androidx.activity.OnBackPressedCallback
import androidx.activity.addCallback
import androidx.fragment.app.Fragment

class EditorFragment : Fragment() {
    private val discard = object : OnBackPressedCallback(true) {
        override fun handleOnBackPressed() {}
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        requireActivity().onBackPressedDispatcher.addCallback(this, discard)
        requireActivity().onBackPressedDispatcher.addCallback(this) { isEnabled = false }
    }
}
