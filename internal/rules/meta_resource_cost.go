// Descriptor metadata for internal/rules/resource_cost.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *BufferedReadWithoutBufferRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BufferedReadWithoutBuffer",
		RuleSet:       "resource-cost",
		DefaultActive: false,
	}
}

func (r *ComposePainterResourceInLoopRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposePainterResourceInLoop",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *ComposeRememberInListRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeRememberInList",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *CursorLoopWithColumnIndexInLoopRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CursorLoopWithColumnIndexInLoop",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *DatabaseQueryOnMainThreadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DatabaseQueryOnMainThread",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *HTTPClientNotReusedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HttpClientNotReused",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *ImageLoadedAtFullSizeInListRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ImageLoadedAtFullSizeInList",
		RuleSet:       "resource-cost",
		DefaultActive: false,
	}
}

func (r *ImageLoaderNoMemoryCacheRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ImageLoaderNoMemoryCache",
		RuleSet:       "resource-cost",
		DefaultActive: false,
	}
}

func (r *LazyColumnInsideColumnRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LazyColumnInsideColumn",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *OkHTTPCallExecuteSyncRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OkHttpCallExecuteSync",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *OkHTTPClientCreatedPerCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OkHttpClientCreatedPerCall",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *PeriodicWorkRequestLessThan15MinRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PeriodicWorkRequestLessThan15Min",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *RecyclerAdapterStableIDsDefaultRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RecyclerAdapterStableIdsDefault",
		RuleSet:       "resource-cost",
		DefaultActive: false,
	}
}

func (r *RecyclerAdapterWithoutDiffUtilRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RecyclerAdapterWithoutDiffUtil",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *RecyclerViewInLazyColumnRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RecyclerViewInLazyColumn",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *RetrofitCreateInHotPathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RetrofitCreateInHotPath",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *RoomLoadsAllWhereFirstUsedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RoomLoadsAllWhereFirstUsed",
		RuleSet:       "resource-cost",
		DefaultActive: true,
	}
}

func (r *WorkManagerNoBackoffRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WorkManagerNoBackoff",
		RuleSet:       "resource-cost",
		DefaultActive: false,
	}
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WorkManagerUniquePolicyKeepButReplaceIntended",
		RuleSet:       "resource-cost",
		DefaultActive: false,
	}
}
