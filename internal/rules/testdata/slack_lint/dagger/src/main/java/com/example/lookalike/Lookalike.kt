package com.example.lookalike

// Local annotations that only share Dagger's simple names.
annotation class Provides
annotation class Binds

class NotModule {
  @Provides fun provide(): String = ""
  @Binds fun bind(): String = ""
}
