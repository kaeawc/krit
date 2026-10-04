// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 14
// The legacy support library Fragment (android.support.v4.app.Fragment),
// declared here because no stub models it. It is a Fragment root the
// framework re-instantiates, and Go matches it by name, so both report.
package android.support.v4.app

open class Fragment

open class DialogFragment : Fragment()

<!FragmentConstructor!>class<!> Legacy(val id: Int) : Fragment()

<!FragmentConstructor!>class<!> LegacyDialog(val id: Int) : DialogFragment()

class LegacyNoArg : Fragment()
