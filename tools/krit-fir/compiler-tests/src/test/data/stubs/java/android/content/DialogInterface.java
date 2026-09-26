// Compiler-test source stub; never packaged in the production artifact.
package android.content;

public interface DialogInterface {
    int BUTTON_POSITIVE = -1;

    int BUTTON_NEGATIVE = -2;

    void cancel();

    void dismiss();

    public static interface OnClickListener {
        void onClick(DialogInterface dialog, int which);
    }

    public static interface OnCancelListener {
        void onCancel(DialogInterface dialog);
    }

    public static interface OnDismissListener {
        void onDismiss(DialogInterface dialog);
    }
}
