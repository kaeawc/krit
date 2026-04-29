package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func runOnClickRule(t *testing.T, code string, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	for _, r := range v2rules.Registry {
		if r.ID != "OnClick" {
			continue
		}
		ctx := &v2rules.Context{
			ParsedFiles:   []*scanner.File{file},
			ResourceIndex: idx,
			Collector:     scanner.NewFindingCollector(0),
			Rule:          r,
		}
		r.Check(ctx)
		return ctx.Collector.Columns().Findings()
	}
	t.Fatal("OnClick rule not registered")
	return nil
}

func onClickIndexWithHandler(layoutName, handler string) *android.ResourceIndex {
	idx := emptyIndex()
	layout := &android.Layout{
		Name:     layoutName,
		FilePath: "res/layout/" + layoutName + ".xml",
		RootView: &android.View{
			Type: "Button",
			Line: 7,
			Attributes: map[string]string{
				"android:onClick": handler,
			},
		},
	}
	idx.Layouts[layoutName] = layout
	idx.LayoutConfigs[layoutName] = map[string]*android.Layout{"": layout}
	return idx
}

func TestOnClickRule_FlagsMissingHandler(t *testing.T) {
	idx := onClickIndexWithHandler("activity_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }
}
`, idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing handler, got %d", len(findings))
	}
}

func TestOnClickRule_FlagsWrongSignature(t *testing.T) {
	idx := onClickIndexWithHandler("activity_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    fun onSubmitClicked() {
        submitForm()
    }
}
`, idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for zero-arg handler, got %d", len(findings))
	}
}

func TestOnClickRule_FlagsPrivateHandler(t *testing.T) {
	idx := onClickIndexWithHandler("activity_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    private fun onSubmitClicked(view: View) {
        submitForm()
    }
}
`, idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for private handler, got %d", len(findings))
	}
}

func TestOnClickRule_FlagsWrongParamType(t *testing.T) {
	idx := onClickIndexWithHandler("activity_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    fun onSubmitClicked(button: Button) {
        submitForm()
    }
}
`, idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for non-View param, got %d", len(findings))
	}
}

func TestOnClickRule_AllowsValidHandler(t *testing.T) {
	idx := onClickIndexWithHandler("activity_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    fun onSubmitClicked(view: View) {
        submitForm()
    }
}
`, idx)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for valid handler, got %d", len(findings))
	}
}

func TestOnClickRule_AllowsValidHandlerViaInflate(t *testing.T) {
	idx := onClickIndexWithHandler("fragment_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class FormFragment : Fragment() {
    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, state: Bundle?): View {
        return inflater.inflate(R.layout.fragment_form, container, false)
    }

    fun onSubmitClicked(view: View) {
        submitForm()
    }
}
`, idx)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for valid handler via inflate, got %d", len(findings))
	}
}

func TestOnClickRule_IgnoresClassesNotInflatingLayout(t *testing.T) {
	idx := onClickIndexWithHandler("activity_form", "onSubmitClicked")
	findings := runOnClickRule(t, `package test
class OtherActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_other)
    }
}
`, idx)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when class doesn't inflate the layout, got %d", len(findings))
	}
}

func TestOnClickRule_NoLayoutOnClick(t *testing.T) {
	idx := emptyIndex()
	layout := &android.Layout{
		Name:     "activity_form",
		FilePath: "res/layout/activity_form.xml",
		RootView: &android.View{Type: "LinearLayout"},
	}
	idx.Layouts["activity_form"] = layout
	idx.LayoutConfigs["activity_form"] = map[string]*android.Layout{"": layout}
	findings := runOnClickRule(t, `package test
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }
}
`, idx)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings without onClick attribute, got %d", len(findings))
	}
}
