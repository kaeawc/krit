package p;

import android.app.Service;
import android.content.Intent;
import android.os.IBinder;

public class JService extends Service {
    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        startActivity(new Intent(this, MainActivity.class));
        return 0;
    }
    @Override
    public IBinder onBind(Intent intent) { return null; }
}
