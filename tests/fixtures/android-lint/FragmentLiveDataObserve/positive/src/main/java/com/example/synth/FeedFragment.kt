package com.example.synth

import android.os.Bundle
import android.view.View
import androidx.fragment.app.Fragment
import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData

class FeedState {
    val items: LiveData<List<String>> = MutableLiveData(emptyList())
}

class FeedFragment : Fragment() {
    private val state = FeedState()
    private val refreshing = MutableLiveData(false)

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        state.items.observe(this) { items -> render(items) }
        view.post { refreshing.observe(this@FeedFragment) { } }
        bindSpinner()
    }

    private fun bindSpinner() {
        refreshing.observe(this) { }
    }

    private fun render(items: List<String>) {}
}
