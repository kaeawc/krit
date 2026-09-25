// Smoke: Anvil contribution annotations on classes with their scope arguments.
package stubs

import com.squareup.anvil.annotations.ContributesBinding
import com.squareup.anvil.annotations.ContributesMultibinding
import com.squareup.anvil.annotations.ContributesSubcomponent
import com.squareup.anvil.annotations.ContributesTo
import com.squareup.anvil.annotations.MergeComponent
import javax.inject.Inject

abstract class AnvilAppScope

abstract class AnvilLoggedInScope

interface AnvilRepo

@ContributesBinding(AnvilAppScope::class)
class RealAnvilRepo @Inject constructor() : AnvilRepo

@ContributesMultibinding(AnvilAppScope::class, boundType = AnvilRepo::class)
class OtherAnvilRepo @Inject constructor() : AnvilRepo

@ContributesTo(AnvilAppScope::class)
interface AnvilBindings {
    fun repo(): AnvilRepo
}

@ContributesSubcomponent(scope = AnvilLoggedInScope::class, parentScope = AnvilAppScope::class)
interface AnvilLoggedInComponent

@MergeComponent(AnvilAppScope::class)
interface AnvilAppComponent
