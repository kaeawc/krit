package com.example.javaonly;

import android.app.Activity;
import android.os.Bundle;
import androidx.room.Dao;
import androidx.room.Query;
import java.util.List;
import java.util.concurrent.Executor;

public final class SignalStyleDatabase extends Activity {
    private final UserDao userDao;
    private final Executor backgroundExecutor;

    public SignalStyleDatabase(UserDao userDao, Executor backgroundExecutor) {
        this.userDao = userDao;
        this.backgroundExecutor = backgroundExecutor;
    }

    @Override
    protected void onResume() {
        super.onResume();
        userDao.loadAllUsers();
        backgroundExecutor.execute(new Runnable() {
            @Override
            public void run() {
                userDao.loadAllUsers();
            }
        });
    }

    @Dao
    interface UserDao {
        @Query("SELECT * FROM users")
        List<User> loadAllUsers();
    }

    static final class User {
    }
}
