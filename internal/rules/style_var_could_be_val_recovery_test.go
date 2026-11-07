package rules_test

import "testing"

func TestVarCouldBeVal_ProtectedConstructorClassBodyMembersAreNotLocals(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test

abstract class GaugeView
protected constructor(
  context: Context,
  attrs: AttributeSet?,
  private val sensor: Sensor,
) : FrameLayout(context, attrs, 0), Calibrated by DefaultCalibration() {

  var isMuted = false

  @get:JvmName("getIsStale") var isStale: Boolean = false

  protected fun sensor(): Sensor {
    return sensor
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("members of a recovered class body are not locals, got %d", len(findings))
	}
}

func TestVarCouldBeVal_ThisQualifiedWriteStrandedByClassBodyRecovery(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test

class TrackRow(
  binding: Binding,
  player: Player,
) : RowBase(binding.root, player), Selectable {

  private var cover: ImageView? = null
  private var queue: Queue? = null

  override fun getCover(): ImageView {
    if (cover == null) {
      cover = stub.inflate()
    }
    return cover!!
  }

  override fun dim(count: Int) {
    require(count >= 0)
    for (i in count until badges.size) {
      badges.getOrNull(i)?.alpha = 0.5f
    }
  }

  override fun setQueue(queue: Queue) {
    this.queue = queue
  }

  override fun getQueue(): Queue? {
    return queue
  }

  @AssistedFactory interface Creator : Creator<TrackRow>

  companion object {
    const val LIMIT = 10
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("a this-qualified write after class body recovery must count, got %d", len(findings))
	}
}

func TestVarCouldBeVal_StillReportsNeverReassignedPrivateAndLocalVars(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test

class Holder {
  private var cache: Cache? = null
  var shared: Int = 0

  fun read(): Cache? {
    var snapshot = cache
    return snapshot
  }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected the private and local vars to be reported, got %d", len(findings))
	}
}
