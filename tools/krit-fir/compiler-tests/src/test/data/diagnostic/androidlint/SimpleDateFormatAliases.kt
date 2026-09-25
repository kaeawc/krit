// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (recall): each call below constructs java.text.SimpleDateFormat
// with the default locale, and FIR reports it. Go misses all of them: it needs
// a call whose callee is spelled SimpleDateFormat, and an import alias, a
// typealias, and a backticked name are other spellings.
package test

import java.text.SimpleDateFormat as Sdf
import java.util.Locale

typealias DateFormatter = java.text.SimpleDateFormat

fun importAlias(): Sdf = <!SimpleDateFormat!>Sdf("yyyy")<!>

fun typeAlias(): DateFormatter = <!SimpleDateFormat!>DateFormatter("yyyy")<!>

fun typeAliasNoArguments(): DateFormatter = <!SimpleDateFormat!>DateFormatter()<!>

fun backticked(): Sdf = <!SimpleDateFormat!>java.text.`SimpleDateFormat`("yyyy")<!>

fun aliasWithLocale(): Sdf = Sdf("yyyy", Locale.US)

fun typeAliasWithLocale(): DateFormatter = DateFormatter("yyyy", Locale.US)
