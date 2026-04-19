package rules_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// =====================================================================
// UnusedResourcesRule tests
// =====================================================================

func TestUnusedResources_FlagsTempPattern(t *testing.T) {
	findings := runRuleByName(t, "UnusedResources", `
package test
fun example() {
    val s = getString(R.string.temp_debug_label)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for temp_ resource pattern")
	}
	if !strings.Contains(findings[0].Message, "temp_debug_label") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestUnusedResources_FlagsTestPattern(t *testing.T) {
	findings := runRuleByName(t, "UnusedResources", `
package test
fun example() {
    val d = getDrawable(R.drawable.test_placeholder)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for test_ resource pattern")
	}
}

func TestUnusedResources_FlagsUnusedPattern(t *testing.T) {
	findings := runRuleByName(t, "UnusedResources", `
package test
fun example() {
    setContentView(R.layout.unused_activity)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused_ resource pattern")
	}
}

func TestUnusedResources_FlagsOldPattern(t *testing.T) {
	findings := runRuleByName(t, "UnusedResources", `
package test
fun example() {
    val c = getColor(R.color.old_primary)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for old_ resource pattern")
	}
}

func TestUnusedResources_IgnoresNormalResources(t *testing.T) {
	findings := runRuleByName(t, "UnusedResources", `
package test
fun example() {
    val s = getString(R.string.app_name)
    val d = getDrawable(R.drawable.ic_launcher)
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for normal resources, got %d", len(findings))
	}
}

func TestUnusedResources_IgnoresComments(t *testing.T) {
	findings := runRuleByName(t, "UnusedResources", `
package test
// val s = getString(R.string.temp_debug)
fun example() {}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for commented-out code, got %d", len(findings))
	}
}

// =====================================================================
// RegisteredRule tests
// =====================================================================

func TestRegistered_FlagsActivity(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
class MyActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unregistered Activity")
	}
	if !strings.Contains(findings[0].Message, "Activity") {
		t.Errorf("expected Activity in message, got: %s", findings[0].Message)
	}
}

func TestRegistered_FlagsService(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
class MyService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unregistered Service")
	}
	if !strings.Contains(findings[0].Message, "Service") {
		t.Errorf("expected Service in message, got: %s", findings[0].Message)
	}
}

func TestRegistered_FlagsBroadcastReceiver(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
class MyReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {}
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unregistered BroadcastReceiver")
	}
}

func TestRegistered_FlagsContentProvider(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
class MyProvider : ContentProvider() {
    override fun onCreate(): Boolean = true
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unregistered ContentProvider")
	}
}

func TestRegistered_SkipsAndroidEntryPoint(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
@AndroidEntryPoint
class MyActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {}
}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for @AndroidEntryPoint, got %d", len(findings))
	}
}

func TestRegistered_SkipsAbstractClass(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
abstract class BaseActivity : AppCompatActivity() {
}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for abstract class, got %d", len(findings))
	}
}

func TestRegistered_IgnoresNonComponent(t *testing.T) {
	findings := runRuleByName(t, "Registered", `
package test
class MyHelper : ViewModel() {
}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for non-component class, got %d", len(findings))
	}
}

// =====================================================================
// LocalSuppressRule tests
// =====================================================================

func TestLocalSuppress_FlagsUnknownIssueID(t *testing.T) {
	findings := runRuleByName(t, "LocalSuppress", `
package test
@SuppressLint("NonExistentLintCheck")
fun example() {}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unknown lint issue ID")
	}
	if !strings.Contains(findings[0].Message, "NonExistentLintCheck") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestLocalSuppress_AcceptsKnownIssueID(t *testing.T) {
	findings := runRuleByName(t, "LocalSuppress", `
package test
@SuppressLint("NewApi")
fun example() {}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for known issue ID, got %d", len(findings))
	}
}

func TestLocalSuppress_AcceptsAll(t *testing.T) {
	findings := runRuleByName(t, "LocalSuppress", `
package test
@SuppressLint("all")
fun example() {}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for 'all', got %d", len(findings))
	}
}

func TestLocalSuppress_IgnoresNonSuppressLint(t *testing.T) {
	findings := runRuleByName(t, "LocalSuppress", `
package test
@Suppress("SomethingElse")
fun example() {}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for @Suppress (not @SuppressLint), got %d", len(findings))
	}
}

// =====================================================================
// SupportAnnotationUsageRule tests
// =====================================================================

func TestSupportAnnotationUsage_FlagsIOInMainThread(t *testing.T) {
	findings := runRuleByName(t, "SupportAnnotationUsage", `
package test
class MyClass {
    @MainThread
    fun loadData() {
        val conn = HttpURLConnection()
        val reader = BufferedReader(input)
    }
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for IO in @MainThread function")
	}
	if !strings.Contains(findings[0].Message, "HttpURLConnection") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestSupportAnnotationUsage_IgnoresNoIO(t *testing.T) {
	findings := runRuleByName(t, "SupportAnnotationUsage", `
package test
class MyClass {
    @MainThread
    fun updateUI() {
        textView.setText("hello")
        progressBar.setVisibility(View.VISIBLE)
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for UI-only @MainThread function, got %d", len(findings))
	}
}

func TestSupportAnnotationUsage_IgnoresNoAnnotation(t *testing.T) {
	findings := runRuleByName(t, "SupportAnnotationUsage", `
package test
class MyClass {
    fun loadData() {
        val conn = HttpURLConnection()
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for function without @MainThread, got %d", len(findings))
	}
}

// =====================================================================
// CustomViewStyleableRule tests
// =====================================================================

func TestCustomViewStyleable_FlagsMismatch(t *testing.T) {
	findings := runRuleByName(t, "CustomViewStyleable", `
package test
class CustomButton : View {
    init {
        val a = context.obtainStyledAttributes(attrs, R.styleable.WrongName)
    }
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for mismatched styleable name")
	}
	if !strings.Contains(findings[0].Message, "WrongName") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCustomViewStyleable_AcceptsMatch(t *testing.T) {
	findings := runRuleByName(t, "CustomViewStyleable", `
package test
class CustomButton : View {
    init {
        val a = context.obtainStyledAttributes(attrs, R.styleable.CustomButton)
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for matching styleable name, got %d", len(findings))
	}
}

func TestCustomViewStyleable_NoClass(t *testing.T) {
	findings := runRuleByName(t, "CustomViewStyleable", `
package test
fun topLevel() {
    val a = context.obtainStyledAttributes(attrs, R.styleable.Something)
}`)
	// No class name found, so no finding expected
	if len(findings) != 0 {
		t.Errorf("expected no finding without class context, got %d", len(findings))
	}
}

// =====================================================================
// ResourceTypeRule tests
// =====================================================================

func TestResourceType_FlagsWrongStringType(t *testing.T) {
	findings := runRuleByName(t, "ResourceType", `
package test
fun example() {
    val s = getString(R.drawable.icon)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for getString(R.drawable.xxx)")
	}
	if !strings.Contains(findings[0].Message, "expected R.string") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestResourceType_FlagsWrongDrawableType(t *testing.T) {
	findings := runRuleByName(t, "ResourceType", `
package test
fun example() {
    val d = getDrawable(R.string.app_name)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for getDrawable(R.string.xxx)")
	}
	if !strings.Contains(findings[0].Message, "expected R.drawable") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestResourceType_FlagsSetImageResource(t *testing.T) {
	findings := runRuleByName(t, "ResourceType", `
package test
fun example() {
    imageView.setImageResource(R.string.app_name)
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for setImageResource(R.string.xxx)")
	}
}

func TestResourceType_AcceptsCorrectType(t *testing.T) {
	findings := runRuleByName(t, "ResourceType", `
package test
fun example() {
    val s = getString(R.string.app_name)
    val d = getDrawable(R.drawable.icon)
    setContentView(R.layout.activity_main)
    val c = getColor(R.color.primary)
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for correct resource types, got %d", len(findings))
	}
}

func TestResourceType_IgnoresComments(t *testing.T) {
	findings := runRuleByName(t, "ResourceType", `
package test
// getString(R.drawable.icon)
fun example() {}`)
	if len(findings) != 0 {
		t.Errorf("expected no finding for commented-out code, got %d", len(findings))
	}
}

// =====================================================================
// IconColorsRule tests
// =====================================================================

func testWriteColoredPNG(t *testing.T, path string, width, height int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating PNG %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding PNG %s: %v", path, err)
	}
}

func testWriteTransparentCornersPNG(t *testing.T, path string, size int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Fill center with solid color, leave corners transparent
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Make a circle-like shape: corners are transparent
			cx, cy := float64(x)-float64(size)/2, float64(y)-float64(size)/2
			r := float64(size) / 2
			if cx*cx+cy*cy < r*r*0.8 {
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			}
			// else stays at zero (transparent)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating PNG %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding PNG %s: %v", path, err)
	}
}

func TestCheckIconColors_FlagsColoredActionBarIcon(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	// Create an action bar icon with predominantly red color
	testWriteColoredPNG(t, filepath.Join(dirPath, "ic_action_share.png"), 24, 24, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconColors(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for colored action bar icon")
	}
	if !strings.Contains(findings[0].Message, "non-standard colors") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconColors_AcceptsWhiteActionBarIcon(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	// Create a white action bar icon
	testWriteColoredPNG(t, filepath.Join(dirPath, "ic_action_share.png"), 24, 24, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconColors(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for white action bar icon, got %d", len(findings))
	}
}

func TestCheckIconColors_AcceptsGrayActionBarIcon(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	testWriteColoredPNG(t, filepath.Join(dirPath, "ic_menu_settings.png"), 24, 24, color.RGBA{R: 128, G: 128, B: 128, A: 255})

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconColors(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for gray action bar icon, got %d", len(findings))
	}
}

func TestCheckIconColors_IgnoresNonActionBarIcon(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	testWriteColoredPNG(t, filepath.Join(dirPath, "bg_splash.png"), 24, 24, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconColors(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-action-bar icon, got %d", len(findings))
	}
}

func TestCheckIconColors_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	rules.CheckIconColors(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// =====================================================================
// IconLauncherShapeRule tests
// =====================================================================

func TestCheckIconLauncherShape_FlagsTransparentCorners(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)

	testWriteTransparentCornersPNG(t, filepath.Join(dirPath, "ic_launcher.png"), 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconLauncherShape(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for launcher icon with transparent corners")
	}
	if !strings.Contains(findings[0].Message, "transparent corners") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconLauncherShape_AcceptsFilledLauncherIcon(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)

	// Fully opaque launcher icon
	testWriteColoredPNG(t, filepath.Join(dirPath, "ic_launcher.png"), 72, 72, color.RGBA{R: 100, G: 150, B: 200, A: 255})

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconLauncherShape(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for filled launcher icon, got %d", len(findings))
	}
}

func TestCheckIconLauncherShape_IgnoresNonLauncherIcon(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)

	testWriteTransparentCornersPNG(t, filepath.Join(dirPath, "ic_share.png"), 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.CheckIconLauncherShape(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-launcher icon, got %d", len(findings))
	}
}

func TestCheckIconLauncherShape_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	rules.CheckIconLauncherShape(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// =====================================================================
// RunAllIconChecks includes new checks
// =====================================================================

func TestRunAllIconChecks_IncludesNewChecks(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Create an action bar icon with red color (should trigger IconColors)
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	testWriteColoredPNG(t, filepath.Join(dirPath, "ic_action_test.png"), 24, 24, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	// Create a launcher icon with transparent corners (should trigger IconLauncherShape)
	testWriteTransparentCornersPNG(t, filepath.Join(dirPath, "ic_launcher.png"), 48)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	rules.RunAllIconChecks(idx, c)
	findings := c.Columns().Findings()
	hasIconColors := false
	hasLauncherShape := false
	for _, f := range findings {
		if f.Rule == "IconColors" {
			hasIconColors = true
		}
		if f.Rule == "IconLauncherShape" {
			hasLauncherShape = true
		}
	}
	if !hasIconColors {
		t.Error("RunAllIconChecks should include IconColors findings")
	}
	if !hasLauncherShape {
		t.Error("RunAllIconChecks should include IconLauncherShape findings")
	}
}
