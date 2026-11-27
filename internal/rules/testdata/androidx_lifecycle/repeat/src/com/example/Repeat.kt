package com.example

import androidx.appcompat.app.AppCompatActivity
import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.launch

class MyFragment : Fragment() {
    override fun onStart() {
        super.onStart()
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {}
        }
    }

    override fun onResume() {
        super.onResume()
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.RESUMED) {} }
        helper()
        helper()
    }

    override fun onViewCreated(view: android.view.View, savedInstanceState: android.os.Bundle?) {
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
    }

    private fun helper() {
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
        deeper()
    }

    private fun deeper() {
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
    }
}

class MyActivity : AppCompatActivity() {
    override fun onStart() {
        super.onStart()
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
        topLevelHelper(this)
    }

    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
    }
}

fun topLevelHelper(activity: AppCompatActivity) {
    activity.lifecycleScope.launch { activity.repeatOnLifecycle(Lifecycle.State.STARTED) {} }
}

class NotLifecycle {
    fun onStart(owner: androidx.lifecycle.LifecycleOwner) {
        owner.lifecycleScope.launch { owner.repeatOnLifecycle(Lifecycle.State.STARTED) {} }
    }
}
