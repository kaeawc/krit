// Descriptor metadata for internal/rules/resource_cost.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *BufferedReadWithoutBufferRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BufferedReadWithoutBuffer",
		RuleSet:       "resource-cost",
		Severity:      "info",
		Description:   "Detects FileInputStream.read() without BufferedInputStream wrapping for efficient reads.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposePainterResourceInLoopRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ComposePainterResourceInLoop",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects painterResource() calls inside list or loop lambdas that create a fresh painter per iteration.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeRememberInListRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ComposeRememberInList",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects remember {} inside items {} without a key argument, causing recomputation on list reorder.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CursorLoopWithColumnIndexInLoopRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CursorLoopWithColumnIndexInLoop",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects getColumnIndex() calls inside cursor while-loops that should be hoisted before the loop.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DatabaseQueryOnMainThreadRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DatabaseQueryOnMainThread",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects SQLiteDatabase query calls in non-suspend functions that may block the main thread.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HttpClientNotReusedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HttpClientNotReused",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects Java HttpClient.newHttpClient() in function bodies without singleton reuse.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ImageLoadedAtFullSizeInListRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ImageLoadedAtFullSizeInList",
		RuleSet:       "resource-cost",
		Severity:      "info",
		Description:   "Detects Glide or Coil image loading without size constraints in list item contexts.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ImageLoaderNoMemoryCacheRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ImageLoaderNoMemoryCache",
		RuleSet:       "resource-cost",
		Severity:      "info",
		Description:   "Detects image loaders configured to skip the memory cache, causing repeated decoding and GC pressure.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LazyColumnInsideColumnRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LazyColumnInsideColumn",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects LazyColumn or LazyRow nested inside a scrollable Column or Row causing measurement issues.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OkHttpCallExecuteSyncRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OkHttpCallExecuteSync",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects synchronous OkHttp Call.execute() inside suspend functions that block the coroutine thread.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OkHttpClientCreatedPerCallRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OkHttpClientCreatedPerCall",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects OkHttpClient construction in function bodies instead of reusing a singleton instance.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PeriodicWorkRequestLessThan15MinRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PeriodicWorkRequestLessThan15Min",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects PeriodicWorkRequest intervals below the 15-minute minimum enforced by WorkManager.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RecyclerAdapterStableIdsDefaultRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RecyclerAdapterStableIdsDefault",
		RuleSet:       "resource-cost",
		Severity:      "info",
		Description:   "Detects RecyclerView.Adapter subclasses that do not enable stable IDs for better animation.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RecyclerAdapterWithoutDiffUtilRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RecyclerAdapterWithoutDiffUtil",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects RecyclerView.Adapter subclasses using notifyDataSetChanged() without DiffUtil.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RecyclerViewInLazyColumnRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RecyclerViewInLazyColumn",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects AndroidView wrapping a RecyclerView inside a LazyColumn or LazyRow causing nested scrolling conflicts.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RetrofitCreateInHotPathRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RetrofitCreateInHotPath",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects Retrofit.Builder().build().create() in function bodies instead of a singleton or @Provides.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RoomLoadsAllWhereFirstUsedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RoomLoadsAllWhereFirstUsed",
		RuleSet:       "resource-cost",
		Severity:      "warning",
		Description:   "Detects getAll().first() patterns that load an entire table for a single element instead of using LIMIT 1.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WorkManagerNoBackoffRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WorkManagerNoBackoff",
		RuleSet:       "resource-cost",
		Severity:      "info",
		Description:   "Detects OneTimeWorkRequest chains without a setBackoffCriteria policy for retryable work.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WorkManagerUniquePolicyKeepButReplaceIntended",
		RuleSet:       "resource-cost",
		Severity:      "info",
		Description:   "Detects enqueueUniqueWork with KEEP policy followed by cancel logic where REPLACE may be intended.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
