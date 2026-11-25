package com.example.synth

import android.os.Bundle
import android.view.View
import android.widget.Toast
import androidx.fragment.app.Fragment

class ProfileFragment : Fragment() {
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        val userId = arguments!!.getString("user")
        Toast.makeText(context!!, userId, Toast.LENGTH_SHORT).show()
        val host = checkNotNull(activity)
    }

    fun close(child: Fragment) {
        child.view!!.visibility = View.GONE
        this.activity!!.finish()
    }
}
