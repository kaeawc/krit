package com.example;

import android.app.Activity;
import android.os.Bundle;
import androidx.recyclerview.widget.RecyclerView;

public class FieldActivity extends Activity {
    private RecyclerView list;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        list = findViewById(R.id.list);
        configure();
    }

    private void configure() {
        list.setHasFixedSize(true);
    }
}
