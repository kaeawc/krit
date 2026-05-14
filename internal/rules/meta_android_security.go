// Descriptor metadata for internal/rules/android_security.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AddJavascriptInterfaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AddJavascriptInterface",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ByteOrderMarkRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ByteOrderMark",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *DrawAllocationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DrawAllocation",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *EasterEggRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EasterEgg",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ExportedContentProviderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExportedContentProvider",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ExportedReceiverRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExportedReceiver",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *FieldGetterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FieldGetter",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *FloatMathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FloatMath",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GetInstanceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GetInstance",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GrantAllUrisRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GrantAllUris",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *UnprotectedDynamicReceiverRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnprotectedDynamicReceiver",
		RuleSet:       "security",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *AllowAllHostnameVerifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AllowAllHostnameVerifier",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *BroadcastReceiverExportedFlagMissingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BroadcastReceiverExportedFlagMissing",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *HandlerLeakRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HandlerLeak",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *RecycleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Recycle",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *SecureRandomRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SecureRandom",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *TrustedServerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TrustedServer",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WorldReadableFilesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WorldReadableFiles",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WorldWriteableFilesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WorldWriteableFiles",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WebViewAllowContentAccessRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewAllowContentAccess",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WebViewAllowFileAccessRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewAllowFileAccess",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WebViewMixedContentAllowAllRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewMixedContentAllowAll",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WebViewUniversalAccessFromFileUrlsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewUniversalAccessFromFileUrls",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WebViewFileAccessFromFileUrlsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewFileAccessFromFileUrls",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WebViewDebuggingEnabledRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewDebuggingEnabled",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WeakMessageDigestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WeakMessageDigest",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *WeakMacAlgorithmRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WeakMacAlgorithm",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *WeakKeySizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WeakKeySize",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *StaticIvRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StaticIv",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *HardcodedSecretKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedSecretKey",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *HardcodedHTTPURLRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedHttpUrl",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *StartActivityWithUntrustedIntentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StartActivityWithUntrustedIntent",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *RsaNoPaddingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RsaNoPadding",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *PrngFromSystemTimeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PrngFromSystemTime",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *OkHTTPDisableSslValidationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OkHttpDisableSslValidation",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *DisableCertificatePinningRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DisableCertificatePinning",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *InsecureTrustManagerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InsecureTrustManager",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *ImplicitPendingIntentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ImplicitPendingIntent",
		RuleSet:       "security",
		DefaultActive: true,
	}
}
