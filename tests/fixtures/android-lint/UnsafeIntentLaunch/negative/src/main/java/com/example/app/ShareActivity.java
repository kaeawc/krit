package com.example.app;

import android.app.Activity;
import android.content.Intent;

public class ShareActivity extends Activity {
    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        startActivity(intent);
    }
}
