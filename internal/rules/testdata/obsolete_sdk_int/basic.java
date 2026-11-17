package p;

import android.annotation.TargetApi;
import android.app.Activity;
import android.app.Fragment;
import android.os.Build;
import android.widget.TextView;
import androidx.annotation.RequiresApi;

public class J1 extends Activity {
    void a() {
        if (Build.VERSION.SDK_INT >= 19) { System.out.println("a"); }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) { }
        if (Build.VERSION.SDK_INT >= 21) { }
        if (Build.VERSION.SDK_INT < 19) { }
        if (Build.VERSION.SDK_INT > 19) { }
        if (Build.VERSION.SDK_INT >= 19 && x()) { }
        boolean b = Build.VERSION.SDK_INT >= 19;
        int c = Build.VERSION.SDK_INT >= 18 ? 1 : 2;
        if (Build.VERSION.SDK_INT == 19) { }
    }
    boolean x() { return true; }
    @TargetApi(19) void t1() {}
    @TargetApi(Build.VERSION_CODES.KITKAT) void t2() {}
    @RequiresApi(19) void t3() {}
    @RequiresApi(api = 18) void t4() {}
    @TargetApi(22) void t5() {}
    void cast() {
        TextView t = (TextView) findViewById(1);
        TextView u = findViewById(2);
        Object o = (Object) findViewById(3);
        TextView v = (TextView) this.findViewById(4);
    }
    public static class F1 extends Fragment {
        public F1(int x) {}
    }
    public static class F2 extends Fragment {
        private F2() {}
    }
    public static class F3 extends Fragment {
        public F3() {}
        public F3(int x) {}
    }
    public static class F4 extends Fragment {
    }
    public class F5 extends Fragment {
    }
    static class F6 extends Fragment {
    }
    public static class F7 extends android.app.ListFragment {
        public F7(String s) {}
    }
    public static class F8 extends androidx.fragment.app.Fragment {
        public F8(String s) {}
    }
    public static class F9 extends F1 {
        public F9() { super(1); }
    }
    public static class F10 extends F1 {
        public F10(int x) { super(x); }
    }
}
