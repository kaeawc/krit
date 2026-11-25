package p

import androidx.compose.ui.text.intl.Locale
import androidx.compose.ui.text.intl.LocaleList

const val TAG = "fr_FR"
const val GOOD_TAG = "fr-FR"

fun tags() {
    val a = Locale("en_US")
    val b = Locale(languageTag = "zh_Hant_TW")
    val c = LocaleList("en_US,ja_JP")
    val d = LocaleList(languageTags = "de_DE")
    val e = Locale(TAG)
    val f = Locale("en" + "_GB")
    val g = Locale(("pt_BR"))
    val ok = Locale("en-US")
    val okConst = Locale(GOOD_TAG)
    val nested = LocaleList(Locale("es_ES"), Locale("it-IT"))
    val local = "ko_KR"
    val viaLocal = Locale(local)
    val suffix = "US"
    val template = Locale("en_$suffix")
    val raw = Locale("""nl_NL""")
    val list = LocaleList(listOf(Locale("sv_SE")))
}
