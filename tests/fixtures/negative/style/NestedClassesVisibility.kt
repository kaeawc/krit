package style

// No explicit public modifier — default visibility is fine
internal class ValidOuter {
    class Nested1
    internal class Nested2
    private class Nested3
    enum class Direction { NORTH, SOUTH }
    internal interface I
    companion object
}

// Public parent — nested public is not misleading
open class PublicOuter {
    public class Nested
}

// Private parent — not flagged
private class PrivateOuter {
    class Nested
}

// Interface parent — not flagged
internal interface OuterInterface {
    class Nested
}

// Sealed class with subtypes — not flagged (parent is not internal, or subtypes lack public)
sealed class Result {
    class Success : Result()
    class Error : Result()
}

// Internal sealed — subtypes without explicit public are fine
internal sealed class InternalResult {
    class Success : InternalResult()
    class Error : InternalResult()
}

// Internal enum class
internal enum class InternalEnum {
    A, B;
    public class Helper  // enum nested class is skipped
}
