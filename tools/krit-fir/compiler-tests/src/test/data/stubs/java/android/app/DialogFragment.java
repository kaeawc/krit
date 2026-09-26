// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.DialogInterface;
import android.os.Bundle;

// Deprecated in API 28. show(FragmentManager, String) is not modeled because
// android.app.FragmentManager is not.
@Deprecated
public class DialogFragment extends Fragment
        implements DialogInterface.OnCancelListener, DialogInterface.OnDismissListener {
    @Deprecated
    public static final int STYLE_NORMAL = 0;

    @Deprecated
    public static final int STYLE_NO_TITLE = 1;

    @Deprecated
    public DialogFragment() {
    }

    @Deprecated
    public void dismiss() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public Dialog getDialog() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void setCancelable(boolean cancelable) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public boolean isCancelable() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public Dialog onCreateDialog(Bundle savedInstanceState) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onCancel(DialogInterface dialog) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public void onDismiss(DialogInterface dialog) {
        throw new RuntimeException("Stub!");
    }
}
