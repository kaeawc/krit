package com.example.javaonly;

import android.app.Activity;
import android.app.Fragment;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.os.Bundle;
import android.webkit.WebView;
import androidx.annotation.CheckResult;
import androidx.fragment.app.FragmentManager;
import java.io.FileInputStream;

public final class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        WebView webView = new WebView(this);
        webView.getSettings().setJavaScriptEnabled(true);
        webView.addJavascriptInterface(new Object(), "bridge");

        SharedPreferences prefs = getSharedPreferences("prefs", MODE_PRIVATE);
        SharedPreferences.Editor editor = prefs.edit();
        editor.putString("token", "value");

        ignoredResult();
        readWithoutBuffer();

        Cursor cursor = getContentResolver().query(null, null, null, null, null);
        while (cursor.moveToNext()) {
            int nameColumn = cursor.getColumnIndex("name");
            cursor.getString(nameColumn);
        }
    }

    void show(FragmentManager manager) {
        manager.beginTransaction().replace(1, new Fragment());
    }

    @CheckResult
    private String ignoredResult() {
        return "value";
    }

    private void readWithoutBuffer() {
        try {
            byte[] bytes = new byte[4096];
            FileInputStream input = new FileInputStream("/tmp/data");
            input.read(bytes);
        } catch (Exception ignored) {
        }
    }
}
