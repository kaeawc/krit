// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.Intent;
import android.os.Bundle;
import android.view.ContextThemeWrapper;
import android.view.Menu;
import android.view.MenuInflater;
import android.view.MenuItem;
import android.view.View;

public class Activity extends ContextThemeWrapper {
    public static final int RESULT_CANCELED = 0;

    public static final int RESULT_OK = -1;

    public static final int RESULT_FIRST_USER = 1;

    public Activity() {
    }

    public Intent getIntent() {
        throw new RuntimeException("Stub!");
    }

    public void setIntent(Intent newIntent) {
        throw new RuntimeException("Stub!");
    }

    public boolean isFinishing() {
        throw new RuntimeException("Stub!");
    }

    public MenuInflater getMenuInflater() {
        throw new RuntimeException("Stub!");
    }

    protected void onCreate(Bundle savedInstanceState) {
        throw new RuntimeException("Stub!");
    }

    protected void onStart() {
        throw new RuntimeException("Stub!");
    }

    protected void onRestart() {
        throw new RuntimeException("Stub!");
    }

    protected void onResume() {
        throw new RuntimeException("Stub!");
    }

    protected void onPause() {
        throw new RuntimeException("Stub!");
    }

    protected void onStop() {
        throw new RuntimeException("Stub!");
    }

    protected void onDestroy() {
        throw new RuntimeException("Stub!");
    }

    protected void onSaveInstanceState(Bundle outState) {
        throw new RuntimeException("Stub!");
    }

    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        throw new RuntimeException("Stub!");
    }

    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onBackPressed() {
        throw new RuntimeException("Stub!");
    }

    public boolean onCreateOptionsMenu(Menu menu) {
        throw new RuntimeException("Stub!");
    }

    public boolean onOptionsItemSelected(MenuItem item) {
        throw new RuntimeException("Stub!");
    }

    public void setContentView(int layoutResID) {
        throw new RuntimeException("Stub!");
    }

    public void setContentView(View view) {
        throw new RuntimeException("Stub!");
    }

    public <T extends View> T findViewById(int id) {
        throw new RuntimeException("Stub!");
    }

    public final <T extends View> T requireViewById(int id) {
        throw new RuntimeException("Stub!");
    }

    public void finish() {
        throw new RuntimeException("Stub!");
    }

    public final void setResult(int resultCode) {
        throw new RuntimeException("Stub!");
    }

    public final void setResult(int resultCode, Intent data) {
        throw new RuntimeException("Stub!");
    }

    public final void runOnUiThread(Runnable action) {
        throw new RuntimeException("Stub!");
    }

    public void startActivityForResult(Intent intent, int requestCode) {
        throw new RuntimeException("Stub!");
    }

    public final void requestPermissions(String[] permissions, int requestCode) {
        throw new RuntimeException("Stub!");
    }
}
