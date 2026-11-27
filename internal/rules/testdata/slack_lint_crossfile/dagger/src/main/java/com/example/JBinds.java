package com.example;

import dagger.Binds;
import dagger.Module;

@Module
abstract class JRepoModule {
  @Binds abstract Repo bindRepo(RepoImpl impl);
  @Binds abstract Repo bindWrong(CacheImpl impl);
}
