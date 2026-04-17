# ViewConstructor

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

Custom `View` subclasses that do not provide the `constructor(context: Context, attrs: AttributeSet)` two-argument constructor. Android's layout inflater invokes this exact constructor when it instantiates views from XML; if it is absent the app will crash with a `NoSuchMethodException` whenever the view is used in a layout file.

## Example — triggers

```kotlin
class RatingBarView(context: Context) : View(context) {
    // Missing (Context, AttributeSet) constructor — unusable from XML layouts
    private var rating: Float = 0f

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        // draw stars…
    }
}
```

## Example — does not trigger

```kotlin
class RatingBarView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0
) : View(context, attrs, defStyleAttr) {

    private var rating: Float = 0f

    init {
        context.theme.obtainStyledAttributes(attrs, R.styleable.RatingBarView, 0, 0).apply {
            rating = getFloat(R.styleable.RatingBarView_rating, 0f)
            recycle()
        }
    }
}
```

## Implementation notes

- Dispatch: `class_declaration`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Medium — detect classes that extend `View` (or any indirect `View` subclass in a configurable list); check declared constructors for a `(Context, AttributeSet)` or `@JvmOverloads (Context, AttributeSet?, Int)` signature; skip abstract classes
- Related: `ViewConstructorDetector` (AOSP), `FragmentConstructor`

## Links

- Parent overview: [`../README.md`](../README.md)
