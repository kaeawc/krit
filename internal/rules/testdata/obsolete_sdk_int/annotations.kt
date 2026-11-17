package p

import android.annotation.TargetApi
import androidx.annotation.RequiresApi

@RequiresApi(23)
class Outer {
    @RequiresApi(19) fun a() {}
    @RequiresApi(23) fun b() {}
    @RequiresApi(24) fun c() {}
    @TargetApi(22) fun d() {}
    @TargetApi(23) fun d2() {}
    @RequiresApi(15) fun d3() {}
    @RequiresApi(21) class Inner {
        @RequiresApi(10) fun e() {}
        @RequiresApi(21) fun e2() {}
        @RequiresApi(25) fun e3() {}
    }
    fun f() {
        @RequiresApi(20) fun g() {}
        val x = object : Any() {
            @RequiresApi(5) fun h() {}
        }
    }
}
@TargetApi(24)
class Outer2 {
    @RequiresApi(22) fun a() {}
    @RequiresApi(25) fun b() {}
}
@RequiresApi(10)
class Outer3 {
    @RequiresApi(5) fun a() {}
    @RequiresApi(15) fun b() {}
    @RequiresApi(30) fun c() {}
}
