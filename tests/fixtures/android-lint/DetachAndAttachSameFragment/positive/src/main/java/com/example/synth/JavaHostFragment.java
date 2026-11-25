package com.example.synth;

import androidx.fragment.app.Fragment;
import androidx.fragment.app.FragmentTransaction;

public class JavaHostFragment extends Fragment {
    void refresh(Fragment page) {
        FragmentTransaction transaction = getParentFragmentManager().beginTransaction();
        transaction.detach(page);
        transaction.attach(page);
        transaction.commit();
    }
}
