package com.example.app;

import android.content.Context;
import android.content.Intent;

class JavaNavigator {
    void openSettings(Context context) {
        Intent intent = new Intent();
        intent.setAction("com.example.app.OPEN_SETTINGS");
        context.startActivity(intent);
    }
}
