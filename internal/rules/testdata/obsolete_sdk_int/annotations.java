package p;

import android.annotation.TargetApi;
import androidx.annotation.RequiresApi;

@RequiresApi(23)
public class J4 {
    @RequiresApi(19) void a() {}
    @TargetApi(19) void b() {}
    @RequiresApi(api = 19) void c() {}
    @RequiresApi(value = 19) void d() {}
    @RequiresApi(19) int f;
    void e(@RequiresApi(19) int p) {}
    @RequiresApi(19) J4(int x) {}
    @RequiresApi(19) static class N {}
}
