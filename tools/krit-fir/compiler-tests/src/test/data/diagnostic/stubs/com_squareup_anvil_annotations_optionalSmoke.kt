// Smoke: Anvil optional SingleIn with its KClass scope argument.
package stubs

import com.squareup.anvil.annotations.optional.SingleIn

class OptionalScope

@SingleIn(OptionalScope::class)
class OptionalAnvilSmoke
