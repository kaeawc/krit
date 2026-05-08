// Convention plugin that consumes a catalog accessor — proves the
// scanConventionPlugins flag picks up references outside module
// build.gradle(.kts) files.
fun applyConvention(target: Any) {
    val coord = "libs.convention.helper"
    println(coord)
}
