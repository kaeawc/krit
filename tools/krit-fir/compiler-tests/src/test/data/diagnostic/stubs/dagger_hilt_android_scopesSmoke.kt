// Smoke: Hilt scopes on non-generic injectable classes and on a @Provides function.
package stubs

import dagger.Module
import dagger.Provides
import dagger.hilt.android.scopes.ActivityRetainedScoped
import dagger.hilt.android.scopes.ActivityScoped
import dagger.hilt.android.scopes.FragmentScoped
import dagger.hilt.android.scopes.ServiceScoped
import dagger.hilt.android.scopes.ViewModelScoped
import dagger.hilt.android.scopes.ViewScoped
import javax.inject.Inject

@ActivityScoped
class ScopedNavigator @Inject constructor()

@ActivityRetainedScoped
class ScopedSession @Inject constructor()

@FragmentScoped
class ScopedFormState @Inject constructor()

@ViewScoped
class ScopedViewBinder @Inject constructor()

@ViewModelScoped
class ScopedRepository @Inject constructor(val session: ScopedSession)

@ServiceScoped
class ScopedPlayer @Inject constructor()

class ScopedFormatter

@Module
object ScopedFormatterModule {
    @Provides
    @ActivityScoped
    fun provideFormatter(): ScopedFormatter = ScopedFormatter()
}
