package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

var lazyListCallNames = map[string]bool{
	"items":          true,
	"itemsIndexed":   true,
	"item":           true,
	"forEach":        true,
	"forEachIndexed": true,
}

func resourceCostInsideLazyListLambda(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "call_expression" {
			callName := flatCallNameAny(file, cur)
			if lazyListCallNames[callName] {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Batch 1: In-Progress rules
// ---------------------------------------------------------------------------

// BufferedReadWithoutBufferRule detects FileInputStream.read(ByteArray(N))
// where N < 8192 without wrapping in BufferedInputStream.
type BufferedReadWithoutBufferRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *BufferedReadWithoutBufferRule) Confidence() float64 { return 0.75 }

// CursorLoopWithColumnIndexInLoopRule detects getColumnIndex() calls inside
// cursor.moveToNext() while loops.
type CursorLoopWithColumnIndexInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CursorLoopWithColumnIndexInLoopRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 2: Network/IO rules
// ---------------------------------------------------------------------------

// OkHttpClientCreatedPerCallRule detects OkHttpClient() or
// OkHttpClient.Builder().build() in non-singleton function bodies.
type OkHttpClientCreatedPerCallRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHttpClientCreatedPerCallRule) Confidence() float64 { return 0.75 }

// OkHttpCallExecuteSyncRule detects Call.execute() inside suspend functions
// where enqueue() with a callback should be used instead.
type OkHttpCallExecuteSyncRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHttpCallExecuteSyncRule) Confidence() float64 { return 0.75 }

// RetrofitCreateInHotPathRule detects Retrofit.Builder()...build().create()
// in non-init, non-object function bodies.
type RetrofitCreateInHotPathRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RetrofitCreateInHotPathRule) Confidence() float64 { return 0.75 }

// HttpClientNotReusedRule detects Java HttpClient.newHttpClient() in function
// bodies without caching.
type HttpClientNotReusedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HttpClientNotReusedRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 3: Database rules
// ---------------------------------------------------------------------------

// DatabaseQueryOnMainThreadRule detects SQLiteDatabase.rawQuery()/query()
// calls in non-suspend functions without withContext.
type DatabaseQueryOnMainThreadRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DatabaseQueryOnMainThreadRule) Confidence() float64 { return 0.75 }

var sqliteQueryMethods = map[string]bool{
	"rawQuery": true,
	"query":    true,
	"execSQL":  true,
}

// RoomLoadsAllWhereFirstUsedRule detects dao.getAll().first() or similar
// patterns that load an entire table for a single element.
type RoomLoadsAllWhereFirstUsedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RoomLoadsAllWhereFirstUsedRule) Confidence() float64 { return 0.75 }

var loadAllTerminalMethods = map[string]bool{
	"first":        true,
	"firstOrNull":  true,
	"single":       true,
	"singleOrNull": true,
	"last":         true,
	"lastOrNull":   true,
}

var loadAllMethods = map[string]bool{
	"getAll":    true,
	"findAll":   true,
	"loadAll":   true,
	"fetchAll":  true,
	"queryAll":  true,
	"selectAll": true,
}

// ---------------------------------------------------------------------------
// Batch 4: RecyclerView/List rules
// ---------------------------------------------------------------------------

// RecyclerAdapterWithoutDiffUtilRule detects RecyclerView.Adapter subclasses
// using notifyDataSetChanged() without DiffUtil or ListAdapter.
type RecyclerAdapterWithoutDiffUtilRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerAdapterWithoutDiffUtilRule) Confidence() float64 { return 0.75 }

// RecyclerAdapterStableIdsDefaultRule detects RecyclerView.Adapter subclasses
// that don't call setHasStableIds(true) and don't extend ListAdapter.
type RecyclerAdapterStableIdsDefaultRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerAdapterStableIdsDefaultRule) Confidence() float64 { return 0.75 }

var lazyColumnToken = []byte("LazyColumn")
var lazyRowToken = []byte("LazyRow")

// LazyColumnInsideColumnRule detects LazyColumn nested inside a Column with
// verticalScroll modifier.
type LazyColumnInsideColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LazyColumnInsideColumnRule) Confidence() float64 { return 0.75 }

// RecyclerViewInLazyColumnRule detects AndroidView wrapping a RecyclerView
// inside a LazyColumn/LazyRow.
type RecyclerViewInLazyColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerViewInLazyColumnRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 5: Image Loading rules
// ---------------------------------------------------------------------------

// ImageLoadedAtFullSizeInListRule detects Glide/Coil image loading without
// size constraints in list item contexts.
type ImageLoadedAtFullSizeInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoadedAtFullSizeInListRule) Confidence() float64 { return 0.75 }

// ImageLoaderNoMemoryCacheRule detects image loaders configured to skip
// the memory cache.
type ImageLoaderNoMemoryCacheRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoaderNoMemoryCacheRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 6: Compose rules
// ---------------------------------------------------------------------------

// ComposePainterResourceInLoopRule detects painterResource() inside
// forEach/items lambda bodies.
type ComposePainterResourceInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposePainterResourceInLoopRule) Confidence() float64 { return 0.75 }

// ComposeRememberInListRule detects remember{} inside items{} lambda
// without a key argument — causes recomputation on list reordering.
type ComposeRememberInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeRememberInListRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// Batch 7: WorkManager rules
// ---------------------------------------------------------------------------

// PeriodicWorkRequestLessThan15MinRule detects PeriodicWorkRequestBuilder
// with an interval less than 15 minutes.
type PeriodicWorkRequestLessThan15MinRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PeriodicWorkRequestLessThan15MinRule) Confidence() float64 { return 0.75 }

// WorkManagerNoBackoffRule detects OneTimeWorkRequestBuilder chains without
// setBackoffCriteria.
type WorkManagerNoBackoffRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerNoBackoffRule) Confidence() float64 { return 0.75 }

// WorkManagerUniquePolicyKeepButReplaceIntendedRule detects enqueueUniqueWork
// with ExistingWorkPolicy.KEEP where REPLACE may be intended.
type WorkManagerUniquePolicyKeepButReplaceIntendedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) Confidence() float64 { return 0.75 }
