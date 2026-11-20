// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 12, 13, 14, 18, 22, 26, 32, 42, 43
package test.plainfile

import java.io.File
import java.io.File as JavaFile

fun paths(dir: File, text: String, bytes: ByteArray) {
    <!PlainFileWriteOfSensitive!>File(dir, "credentials.json").writeText(text)<!>
    <!PlainFileWriteOfSensitive!>File(dir, "auth_token.bin").writeBytes(bytes)<!>
    <!PlainFileWriteOfSensitive!>File("userPassword.txt").writeText(text)<!>
    <!PlainFileWriteOfSensitive!>File("private-key.pem").writeBytes(bytes)<!>
    <!PlainFileWriteOfSensitive!>File("OAUTH.json").writeText(text)<!>
    <!PlainFileWriteOfSensitive!>File("token_expires_at.txt").writeText(text)<!>
}

fun raw(text: String) {
    <!PlainFileWriteOfSensitive!>File("""client_secret.txt""").writeText(text)<!>
}

fun template(dir: File, userId: String, text: String) {
    <!PlainFileWriteOfSensitive!>File(dir, "token_$userId.txt").writeText(text)<!>
}

fun multiline(text: String) {
    <!PlainFileWriteOfSensitive!>File("passwords.txt").writeText<!>(
        text,
    )
}

fun firstLiteral(text: String) {
    <!PlainFileWriteOfSensitive!>File("secrets", "cache.txt").writeText(text)<!>
}

fun alias(text: String) {
    <!PlainFileWriteOfSensitive!>JavaFile("credentials.json").writeText(text)<!>
}

fun identity(file: File): File = file

fun wrapped(text: String) {
    <!PlainFileWriteOfSensitive!>identity(File("tokens.txt")).writeText(text)<!>
    <!PlainFileWriteOfSensitive!>File("secret.txt").absoluteFile.writeText(text)<!>
}
