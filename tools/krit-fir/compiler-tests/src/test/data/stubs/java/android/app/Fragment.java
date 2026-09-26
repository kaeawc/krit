// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.Context;
import android.content.res.Resources;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;

// Deprecated in API 28, class and members alike. The SDK class also implements
// ComponentCallbacks2 and View.OnCreateContextMenuListener, which are not
// modeled; android.app.FragmentManager is not modeled either.
@Deprecated
public class Fragment {
    @Deprecated
    public Fragment() {
    }

    @Deprecated
    public static Fragment instantiate(Context context, String fname) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public static Fragment instantiate(Context context, String fname, Bundle args) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void setArguments(Bundle args) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public final Bundle getArguments() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public Context getContext() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public final Activity getActivity() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public final Resources getResources() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public final String getString(int resId) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public View getView() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onAttach(Context context) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onCreate(Bundle savedInstanceState) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public View onCreateView(LayoutInflater inflater, ViewGroup container, Bundle savedInstanceState) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onViewCreated(View view, Bundle savedInstanceState) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onStart() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onResume() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onSaveInstanceState(Bundle outState) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onPause() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onStop() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onDestroyView() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onDestroy() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onDetach() {
        throw new RuntimeException("Stub!");
    }
}
