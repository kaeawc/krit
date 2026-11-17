package android.content;

public class ClipboardManager extends android.text.ClipboardManager {
    ClipboardManager() {
    }

    public boolean hasPrimaryClip() {
        return false;
    }

    public void clearPrimaryClip() {
    }

    @Deprecated
    public CharSequence getText() {
        return null;
    }

    @Deprecated
    public void setText(CharSequence text) {
    }

    @Deprecated
    public boolean hasText() {
        return false;
    }
}
