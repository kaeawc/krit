// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvement: an annotation or a label on the literal argument does
// not change its value, so these calls are real findings. Go misses them because
// it sees an annotated_expression or labeled_expression instead of a
// string_literal; FIR is correct because it reads the literal itself.
package test

import javax.crypto.Cipher

class Crypto {
    fun annotated(): Cipher = <!RsaNoPadding!>Cipher.getInstance(@Suppress("x") "RSA/ECB/NoPadding")<!>

    fun labeled(): Cipher = <!RsaNoPadding!>Cipher.getInstance(lbl@ "RSA/ECB/NoPadding")<!>
}
