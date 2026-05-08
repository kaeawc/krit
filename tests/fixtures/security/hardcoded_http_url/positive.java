package test;

import java.net.URL;
import okhttp3.Request;
import retrofit2.Retrofit;

class HardcodedHttpUrls {
    void build() throws Exception {
        new Retrofit.Builder().baseUrl("http://api.example.com/").build();
        new Request.Builder().url("http://cdn.example.com/file").build();
        new URL("http://files.example.com/data");
    }
}
