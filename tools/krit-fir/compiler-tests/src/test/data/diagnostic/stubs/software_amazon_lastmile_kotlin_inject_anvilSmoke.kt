// Smoke: kotlin-inject-anvil scoped and contributed bindings.
package stubs

import software.amazon.lastmile.kotlin.inject.anvil.ContributesBinding
import software.amazon.lastmile.kotlin.inject.anvil.SingleIn

class AmazonAppScope

interface AmazonService

@SingleIn(AmazonAppScope::class)
@ContributesBinding(AmazonAppScope::class)
class AmazonServiceImpl : AmazonService
