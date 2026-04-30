package com.example.javaonly;

import android.app.Activity;
import android.app.Fragment;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.webkit.WebView;
import androidx.annotation.CheckResult;
import java.io.BufferedInputStream;
import java.io.FileInputStream;

public final class SafeJavaPaths extends Activity {
    void safe(WebView webView, SharedPreferences prefs, Cursor cursor) throws Exception {
        webView.getSettings().setJavaScriptEnabled(false);
        prefs.edit().putString("token", "value").apply();
        getFragmentManager().beginTransaction().replace(1, new Fragment()).commit();

        String value = checkedValue();
        if (value.isEmpty()) {
            return;
        }

        byte[] bytes = new byte[4096];
        BufferedInputStream input = new BufferedInputStream(new FileInputStream("/tmp/data"));
        input.read(bytes);

        int nameColumn = cursor.getColumnIndex("name");
        while (cursor.moveToNext()) {
            cursor.getString(nameColumn);
        }
    }

    @CheckResult
    private String checkedValue() {
        return "value";
    }
}
