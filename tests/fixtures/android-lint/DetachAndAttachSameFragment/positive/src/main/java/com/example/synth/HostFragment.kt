package com.example.synth

import androidx.fragment.app.Fragment

class HostFragment : Fragment() {
    fun refresh(page: Fragment) {
        parentFragmentManager.beginTransaction()
            .detach(page)
            .attach(page)
            .commit()
    }

    fun reload(page: Fragment) {
        val transaction = childFragmentManager.beginTransaction()
        transaction.detach(page)
        transaction.attach(page)
        transaction.commitNow()
    }
}
