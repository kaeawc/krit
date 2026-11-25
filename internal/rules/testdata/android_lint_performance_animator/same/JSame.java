package p;

import android.animation.ObjectAnimator;
import android.content.Context;
import android.view.View;
import androidx.annotation.Keep;

public class JSame extends View {
    public JSame(Context c) { super(c); }
    public void setLevel(float l) {}
    @Keep public void setSafe(float l) {}
    public void setCount(int c) {}

    void animate() {
        ObjectAnimator.ofFloat(this, "level", 0f, 1f);
        ObjectAnimator.ofFloat(this, "safe", 0f, 1f);
        ObjectAnimator.ofInt(this, "count", 0, 1);
        ObjectAnimator.ofFloat(this, "level", 1f, 0f);
    }
}
