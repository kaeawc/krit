package com.example.synth

import android.database.sqlite.SQLiteDatabase
import androidx.fragment.app.Fragment

class HostFragment : Fragment() {
    fun refresh(page: Fragment) {
        parentFragmentManager.beginTransaction().detach(page).commit()
        parentFragmentManager.beginTransaction().attach(page).commit()
    }

    fun swap(old: Fragment, new: Fragment) {
        parentFragmentManager.beginTransaction().detach(old).attach(new).commit()
    }

    fun migrate(db: SQLiteDatabase) {
        db.beginTransaction()
        db.endTransaction()
    }
}
