// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 29, 32, 35, 38, 41, 44
// Classes extending lookalikes named like the Fragment base classes Go
// matches. Go matches the direct supertype's simple name, so it reports every
// subclass below; none extends an Android Fragment, the framework never
// re-instantiates them, and none is reported here.
package test

open class Fragment(val tag: String = "")

open class DialogFragment

open class ListFragment

open class PreferenceFragment

open class PreferenceFragmentCompat

open class BottomSheetDialogFragment

object Lib {
    open class Fragment
}

// Go reports this because the supertype is named Fragment; it is test.Fragment.
class TextFragment(val text: String) : Fragment()

// Go reports this because the supertype is named DialogFragment.
class LocalDialog(val title: String) : DialogFragment()

// Go reports this because the supertype is named ListFragment.
class LocalList(val items: List<String>) : ListFragment()

// Go reports this because the supertype is named PreferenceFragment.
class LocalPreferences(val key: String) : PreferenceFragment()

// Go reports this because the supertype is named PreferenceFragmentCompat.
class LocalPreferencesCompat(val key: String) : PreferenceFragmentCompat()

// Go reports this because the supertype is named BottomSheetDialogFragment.
class LocalSheet(val key: String) : BottomSheetDialogFragment()

// Go reports this because the qualified supertype's last name is Fragment.
class NestedLookalike(val key: String) : Lib.Fragment()
