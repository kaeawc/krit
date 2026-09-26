// Compiler-test source stubs; never packaged in the production artifact.
// Hilt's Android scopes are Java `@Scope @Retention(CLASS)` annotations with no
// @Target, so they apply to every declaration kind.
package dagger.hilt.android.scopes

import javax.inject.Scope

@Scope
@Retention(AnnotationRetention.BINARY)
annotation class ActivityScoped

@Scope
@Retention(AnnotationRetention.BINARY)
annotation class ActivityRetainedScoped

@Scope
@Retention(AnnotationRetention.BINARY)
annotation class FragmentScoped

@Scope
@Retention(AnnotationRetention.BINARY)
annotation class ViewScoped

@Scope
@Retention(AnnotationRetention.BINARY)
annotation class ViewModelScoped

@Scope
@Retention(AnnotationRetention.BINARY)
annotation class ServiceScoped
