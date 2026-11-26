package com.example;

import android.app.Activity;
import androidx.recyclerview.widget.RecyclerView;

public class JActivity extends Activity {
    void bind() {
        RecyclerView list = findViewById(R.id.list);
        list.setHasFixedSize(true);
        ((RecyclerView) findViewById(R.id.grid)).setHasFixedSize(true);
    }
}
