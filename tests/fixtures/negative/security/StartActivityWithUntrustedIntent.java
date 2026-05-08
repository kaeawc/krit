package test;

import android.app.Activity;
import android.content.Intent;

class Screen extends Activity {
    void guarded(String uri, Intent param) throws Exception {
        Intent intent = Intent.parseUri(uri, 0);
        intent.setComponent(null);
        startActivity(intent);
        Intent intent2 = new Intent();
        startActivity(intent2);
        startActivity(param);
    }
}
