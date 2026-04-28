package potentialbugs

import org.whispersystems.signalservice.internal.push.SyncMessage

class UnsafeCallOnNullableType {
    fun example(nullable: String?) {
        val len = nullable?.length
    }

    // String literal containing !! should not trigger
    fun message() {
        val msg = "use !! to force"
    }

    // a!!.b — only a!! fires, not a!!.b as the outer node
    fun chainedAccess(a: String?) {
        // This negative fixture verifies that navigation on a postfix
        // expression is not double-reported at the navigation_expression level.
        val x = a?.length
    }
}

class PostFilterSafePatterns {
    data class Item(val name: String?, val age: Int?)

    // Same field checked in filter lambda (implicit "it")
    fun safeWithFilter(list: List<Item>): List<String?> {
        return list.filter { it.name != null }.map { it.name!! }
    }

    // filterNotNull guarantees elements are non-null
    fun safeWithFilterNotNull(list: List<String?>): List<String> {
        return list.filterNotNull().map { it!! }
    }

    // Named lambda parameter in filter, same field checked
    fun safeWithNamedParam(list: List<Item>): List<String?> {
        return list.filter { item -> item.name != null }.map { it.name!! }
    }
}

class SignalInspiredSafePatterns {
    class Controller {
        fun start() {}
    }

    class Matcher {
        fun group(index: Int): String? = null
    }

    private var controller: Controller? = null

    fun sameExpressionGuard(flags: Int?): Boolean {
        return flags != null && flags!! and 1 != 0
    }

    fun stableRepeatedMatcherGroup(matcher: Matcher): Boolean {
        return matcher.group(1) != null && matcher.group(1)!!.isNotEmpty()
    }

    fun requireController(): Controller = controller!!

    fun assignedBeforeUse() {
        if (controller == null) {
            controller = Controller()
        }
        controller!!.start()
    }
}

class AndroidCompatSafePatterns {
    class Intent {
        fun getBundleExtra(key: String): Bundle? = null
        fun <T> getParcelableExtraCompat(key: String, clazz: Class<T>): T? = null
        fun <T> getParcelableArrayListExtraCompat(key: String, clazz: Class<T>): ArrayList<T>? = null
    }

    class Bundle
    class Args

    fun read(intent: Intent) {
        val bundle = intent.getBundleExtra("args")!!
        val args = intent.getParcelableExtraCompat("args", Args::class.java)!!
        val list = intent.getParcelableArrayListExtraCompat("args", Args::class.java)!!
    }
}

class ProtoSafePatterns {
    class GroupV2
    class Message(val groupV2: GroupV2?)
    class Sent(val message: Message?)
    class SyncMessage(val sent: Sent?)

    fun group(syncMessage: SyncMessage): GroupV2 {
        return syncMessage.sent!!.message!!.groupV2!!
    }

    fun hasFlag(flags: Int?): Boolean {
        return flags != null && flags!! and 1 != 0
    }
}
