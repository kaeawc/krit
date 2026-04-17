# Android Lint — Resource Sub-Cluster

Rules that analyze XML layout and resource files. Implemented via the `ResourceRule` interface in the resource pipeline.

**Status: 78 shipped, 31 planned**

---

## Shipped Rules (78)

Shipped rules span accessibility, correctness, IDs, layout, RTL, style, and values. Key examples:

| Category | Rule IDs (sample) |
|---|---|
| Accessibility | ContentDescription, ClickableViewAccessibility, ImportantForAccessibility, LabelFor, RepeatedContentDescription, SpanInContentDescription |
| Correctness | HardcodedText, IllegalResourceRef, InconsistentArrays, InconsistentLayout, InvalidId, InvalidResourceFolder, MissingId, MissingPrefix, MissingQuantity, ObsoleteLayoutParam, OnClick, StringFormatCount, StringFormatInvalid, StringFormatMatches, StringFormatTrivial, StringNotLocalizable, UnusedAttribute, UnusedNamespace, UnusedQuantity, WrongCase, WrongFolder, WrongRegion |
| IDs | CutPasteId, DuplicateIds, DuplicateIncludedIds, NotSibling |
| Layout | DisableBaselineAlignment, ExtraText, InefficientWeight, IncludeLayoutParam, MergeRootFrame, NestedScrolling, NestedWeights, Orientation, Overdraw, RelativeOverlap, ScrollViewCount, ScrollViewSize, Suspicious0dp, TooDeepLayout, TooManyViews, UseCompoundDrawables, UselessLeaf, UselessParent, WebViewLayout |
| RTL | RtlHardcoded, RtlSuperscript, RtlSymmetry |
| Style | AlwaysShowAction, AppCompatResource, AutofillHintMismatch, AutofillImportance, BackButton, ButtonCase, ButtonOrder, ButtonStyle, ClickableMinSize, ImpliedQuantity, InOrMmUsage, MinTouchTargetInButtonRow, NegativeMargin, PxUsage, RequiredSize, SmallSp, SpUsage, TextFields, TextNotSelectable |
| Values | ImpliedQuantity, LocaleConfigStale, StateListReachable |

---

## Planned Rules (31)

These AOSP rules are not yet covered. All require XML layout/resource parser infrastructure beyond what the current `ResourceRule` pipeline exposes.

| Rule ID | AOSP Detector | Description |
|---|---|---|
| AdapterViewChildren | ChildCountDetector.ADAPTER_VIEW_ISSUE | AdapterView children detected in XML (should be inflated at runtime) |
| MissingNamespace | DetectMissingPrefix.MISSING_NAMESPACE | Missing namespace declaration on custom attributes |
| DosLineEndings | DosLineEndingDetector | Windows-style CRLF line endings in resource files |
| DuplicateResource | DuplicateResourceDetector.ISSUE | Same resource name defined in multiple files |
| ResourceTypeMismatch | DuplicateResourceDetector.TYPE_MISMATCH | Resource type redefined with a different type |
| GridLayout | GridLayoutDetector | GridLayout with mismatched row/column spans |
| IncludeLayout | IncludeDetector | `<include>` layout_width/height without match_parent |
| InefficientOrientation | InefficientWeightDetector.ORIENTATION | LinearLayout with single child and weight (use match_parent) |
| Wrong0dp | InefficientWeightDetector.WRONG_0DP | 0dp dimension without layout_weight set |
| DeprecatedLocaleCode | LocaleFolderDetector.DEPRECATED_CODE | Deprecated ISO 639 language code in resource folder name |
| UseAlpha2Code | LocaleFolderDetector.USE_ALPHA_2 | Three-letter language code where two-letter code exists |
| CustomViewNamespace | NamespaceDetector.CUSTOM_VIEW | Missing namespace for custom view attributes |
| NamespaceTypo | NamespaceDetector.TYPO | Typo in namespace URI |
| NfcTechList | NfcTechListDetector | Malformed NFC tech-list resource |
| DpInsteadOfPx | PxUsageDetector.DP_ISSUE | Physical pixel unit where dp is appropriate |
| InMmUsage | PxUsageDetector.IN_MM_ISSUE | Inches/mm units in layout (should use dp) |
| PxUsage | PxUsageDetector.PX_ISSUE | Hardcoded px value (should use dp or sp) |
| StateList | StateListDetector | Unreachable state in state list drawable |
| TitleMissing | TitleDetector | Dialog/Toolbar missing required title attribute |
| TooManyViews | TooManyViewsDetector | Layout with excessive number of views |
| Typos | TypoDetector | Common spelling errors in string resources |
| TypographyDashes | TypographyDetector.DASHES | ASCII hyphens where em/en dash is appropriate |
| TypographyEllipsis | TypographyDetector.ELLIPSIS | Three periods where ellipsis character is appropriate |
| TypographyFractions | TypographyDetector.FRACTIONS | ASCII fractions where fraction character is appropriate |
| TypographyOther | TypographyDetector.OTHER | Miscellaneous typography issues |
| TypographyQuotes | TypographyDetector.QUOTES | Straight quotes where curly quotes are appropriate |
| UselessView | UselessViewDetector | View with no effect on layout |
| WebViewLayout | WebViewDetector | WebView inside ScrollView causes sizing issues |
| InvalidId | WrongIdDetector.INVALID | Malformed resource ID reference |
| UnknownId | WrongIdDetector.UNKNOWN_ID | Reference to undefined resource ID |
| UnknownIdLayout | WrongIdDetector.UNKNOWN_ID_LAYOUT | ID referenced in layout not found in same layout |

All 31 require XML layout/resource parser infrastructure: specifically, multi-file resource index support and cross-file ID resolution that the current pipeline does not yet expose.
