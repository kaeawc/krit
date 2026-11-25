package com.example.synth;

import android.os.Bundle;
import android.view.View;
import androidx.fragment.app.Fragment;
import androidx.lifecycle.MutableLiveData;

public class JavaFeedFragment extends Fragment {
    private final MutableLiveData<String> title = new MutableLiveData<>();

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        super.onViewCreated(view, savedInstanceState);
        title.observe(this, value -> { });
    }
}
