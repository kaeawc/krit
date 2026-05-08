package test;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;

class UnsafeGson {
    void parse(String raw) {
        new Gson().fromJson(raw, Object.class);
        new GsonBuilder().create().fromJson(raw, java.lang.Object.class);
    }
}
