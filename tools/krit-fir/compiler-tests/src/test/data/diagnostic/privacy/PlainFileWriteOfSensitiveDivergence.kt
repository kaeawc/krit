// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 13, 15, 20, 22, 24, 26
package test.plainfiledivergence

import java.io.File
import java.nio.file.Paths
import kotlin.io.path.writeText

fun templateOnlyNames(dir: File, tokenFile: String, authDir: String, _token: String, text: String) {
    // Go reports this interpolation-only name; FIR keeps no literal filename text.
    File(dir, "$tokenFile").writeText(text)
    // Go reports this interpolation-only name; FIR keeps no literal filename text.
    File(dir, "${authDir}").writeText(text)
    // Go reports this Kotlin template name; its scanner treats the dollar as literal.
    File(dir, "$_token.txt").writeText(text)
}

fun chainedPathWrites(dir: File, text: String) {
    // Go finds File in the receiver chain; FIR stops at Path.normalize.
    File(dir, "token.txt").toPath().normalize().writeText(text)
    // Go finds File in the receiver chain; FIR stops at Path.toAbsolutePath.
    File(dir, "token.txt").toPath().toAbsolutePath().writeText(text)
    // Go finds File in the receiver chain; FIR stops at Path.resolve.
    File(dir, "token.txt").toPath().resolve("child").writeText(text)
    // Go finds File inside Paths.get; FIR sees only the resulting Path receiver.
    Paths.get(File(dir, "token.txt").path).writeText(text)
}
