package fixtures.positive.slacklint

import dagger.Provides

class NotAModule {
    @Provides fun provideName(): String = "name"
}
