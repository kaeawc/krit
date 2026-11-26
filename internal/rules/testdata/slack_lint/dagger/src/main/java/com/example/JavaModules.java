package com.example;

import dagger.Binds;
import dagger.Module;
import dagger.Provides;

@Module
public abstract class JavaModules {
  @Binds Foo bindConcrete(FooImpl impl) { return impl; }
  @Provides abstract Foo provideAbstract();
  @Binds abstract Foo bindOk(FooImpl impl);
  @Provides static void provideVoid() {}
  @Provides static Foo provideOk() { return null; }
  @Binds abstract Foo bindNone();
  @Binds abstract void bindVoid(FooImpl impl);

  @Module
  interface Inner {
    @Binds default Foo bindDefault(FooImpl impl) { return impl; }
    @Provides Foo provideInInterface();
    @Binds Foo bindOk(FooImpl impl);
  }

  interface NotAModuleInterface {
    @Binds Foo bindOutsideModule(FooImpl impl);
  }

  Object anonymous = new Object() {
    @Provides Foo inAnonymousClass() { return null; }
  };
}

class JavaNotModule {
  /** Javadoc: Lint starts the location here. */
  @Provides static Foo provide() { return null; }

  @Override
  @Provides
  public String toString() { return ""; }

  @SuppressWarnings("MustBeInModule")
  @Provides static Foo suppressed() { return null; }
}

enum JavaEnum {
  A;

  @Provides Foo inEnum() { return null; }
}
