package fixtures.androidlint

import android.content.Context
import androidx.startup.Initializer

// The rule needs the project's manifests, so this negative fixture only holds shapes it never reports without one:
// an interface Initializer, and a class with a startup-like name that does not implement androidx.startup.Initializer.
// The parity case under internal/rules/testdata/androidx_startup/ covers listed and reachable Initializers.
interface StartupStep : Initializer<Unit>

class FakeInitializer {
    fun create(context: Context) = Unit
}
