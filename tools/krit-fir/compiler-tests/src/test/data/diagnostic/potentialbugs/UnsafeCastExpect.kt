// go-lines: 7, 10
package unsafecastexpect

expect class PlatformText

// Go reports this fallback case; FIR abstains because an actual typealias may make the cast valid on JVM.
fun commonCast(value: String) = value as PlatformText

// The expect class is the source here; its actual typealias is still JVM String.
fun commonSourceCast(value: PlatformText) = value as String

expect open class PlatformBase()
interface PlatformMarker
class CommonImpl : PlatformBase()

// FIR abstains because PlatformMarker is open in the common session; the JVM actual adds it through the expect base.
fun commonFinalToPlatformMarker(value: CommonImpl) = value as PlatformMarker
fun platformMarkerToCommonFinal(value: PlatformMarker) = value as CommonImpl
