package com.example.synth

import android.os.Bundle
import android.view.View
import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.MutableLiveData

class EventBus {
    fun observe(owner: LifecycleOwner, block: () -> Unit) {}
}

class FeedFragment : Fragment() {
    private val refreshing = MutableLiveData(false)
    private val bus = EventBus()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // The fragment's own lifecycle is the right owner before the view exists.
        refreshing.observe(this) { }
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        refreshing.observe(viewLifecycleOwner) { }
        // Not LiveData.observe.
        bus.observe(this) { }
        view.post(object : Runnable {
            override fun run() {
                refreshing.observeForever { }
            }
        })
    }
}

class ConfirmDialog : DialogFragment() {
    private val choice = MutableLiveData(0)

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        choice.observe(this) { }
    }
}
