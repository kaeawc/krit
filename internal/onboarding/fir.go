package onboarding

// GoOnlyBaselineNotice explains how to keep later scans consistent with an
// onboarding baseline produced when the FIR requirements are unavailable.
const GoOnlyBaselineNotice = "Baseline is Go-only. FIR needs Java 21+ plus a Gradle model or an oracle.classpath (or CLASSPATH) declaration. Regenerate the baseline after enabling FIR, or pass --no-fir on your scans to stay consistent with this baseline."
