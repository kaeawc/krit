package com.example.synth;

import android.os.Bundle;
import android.view.View;
import androidx.core.view.MenuProvider;
import androidx.fragment.app.Fragment;

public class JavaItemsFragment extends Fragment {
    private MenuProvider menu;

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        super.onViewCreated(view, savedInstanceState);
        requireActivity().addMenuProvider(menu, this);
    }
}
