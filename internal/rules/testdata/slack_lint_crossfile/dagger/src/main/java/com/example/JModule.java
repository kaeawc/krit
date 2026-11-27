package com.example;

import dagger.Module;
import dagger.Provides;

@Module
class JModule {
  @Provides static String provideJ() { return ""; }
}

class JUser {
  void use() {
    JModule.provideJ();
    String s = FooModule.INSTANCE.provideFoo().toString();
  }
}

@javax.annotation.Generated("dagger")
final class JModule_ProvideJFactory {
  String get() { return JModule.provideJ(); }
}

class JInstanceUser {
  void use(InstanceModule module) {
    module.provideCount();
  }
}
