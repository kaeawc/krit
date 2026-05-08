package test;

import android.app.Activity;
import android.content.Intent;

class Screen extends Activity {
    void launch(String uri) throws Exception {
        Intent intent = Intent.parseUri(uri, 0);
        startActivity(intent);
    }
}
