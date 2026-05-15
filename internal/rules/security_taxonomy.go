package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// securityTaxonomyByRuleID maps registered security rule IDs to their
// published taxonomy identifiers. Centralized here so the wiring is
// readable in one place; an init() hook applies the mapping to the
// registry after all rules have been registered.
//
// Every rule whose Category is "security" must appear in this map with
// at least one CWE. The applySecurityTaxonomy hook validates this
// invariant in tests.
var securityTaxonomyByRuleID = map[string]*api.SecurityTaxonomy{
	// Injection / unsanitized input
	"ContentProviderQueryWithSelectionInterpolation": {
		CWE:     []string{"CWE-89"},
		OWASP:   []string{"A03:2021-Injection"},
		SEICert: []string{"IDS00-J"},
	},
	"SqlInjectionRawQuery": {
		CWE:     []string{"CWE-89"},
		OWASP:   []string{"A03:2021-Injection"},
		SEICert: []string{"IDS00-J"},
	},
	"JdbcStatementExecute": {
		CWE:     []string{"CWE-89"},
		OWASP:   []string{"A03:2021-Injection"},
		SEICert: []string{"IDS00-J"},
	},
	"RoomRawQueryStringConcat": {
		CWE:   []string{"CWE-89"},
		OWASP: []string{"A03:2021-Injection"},
	},
	"RuntimeExecUnsafeShape": {
		CWE:     []string{"CWE-78"},
		OWASP:   []string{"A03:2021-Injection"},
		SEICert: []string{"IDS07-J"},
		Mitre:   []string{"T1059"},
	},
	"ProcessBuilderShellArg": {
		CWE:     []string{"CWE-78"},
		OWASP:   []string{"A03:2021-Injection"},
		SEICert: []string{"IDS07-J"},
		Mitre:   []string{"T1059"},
	},

	// Path traversal / zip slip
	"FileFromUntrustedPath": {
		CWE:     []string{"CWE-22"},
		OWASP:   []string{"A01:2021-Broken Access Control"},
		SEICert: []string{"FIO16-J"},
	},
	"ZipSlipUnchecked": {
		CWE:     []string{"CWE-22"},
		OWASP:   []string{"A01:2021-Broken Access Control"},
		SEICert: []string{"FIO16-J"},
	},

	// Deserialization
	"JavaObjectInputStream": {
		CWE:     []string{"CWE-502"},
		OWASP:   []string{"A08:2021-Software and Data Integrity Failures"},
		SEICert: []string{"SER12-J"},
	},
	"JacksonDefaultTyping": {
		CWE:   []string{"CWE-502"},
		OWASP: []string{"A08:2021-Software and Data Integrity Failures"},
	},
	"GsonPolymorphicFromJson": {
		CWE:   []string{"CWE-502"},
		OWASP: []string{"A08:2021-Software and Data Integrity Failures"},
	},

	// XML / XXE
	"XmlExternalEntity": {
		CWE:     []string{"CWE-611"},
		OWASP:   []string{"A05:2021-Security Misconfiguration"},
		SEICert: []string{"IDS17-J"},
	},

	// Cryptography
	"WeakMessageDigest": {
		CWE:     []string{"CWE-327", "CWE-328"},
		OWASP:   []string{"A02:2021-Cryptographic Failures"},
		SEICert: []string{"MSC61-J"},
	},
	"WeakMacAlgorithm": {
		CWE:   []string{"CWE-327"},
		OWASP: []string{"A02:2021-Cryptographic Failures"},
	},
	"WeakKeySize": {
		CWE:   []string{"CWE-326"},
		OWASP: []string{"A02:2021-Cryptographic Failures"},
	},
	"RsaNoPadding": {
		CWE:   []string{"CWE-780"},
		OWASP: []string{"A02:2021-Cryptographic Failures"},
	},
	"StaticIv": {
		CWE:   []string{"CWE-329", "CWE-330"},
		OWASP: []string{"A02:2021-Cryptographic Failures"},
	},
	"PrngFromSystemTime": {
		CWE:     []string{"CWE-330", "CWE-338"},
		OWASP:   []string{"A02:2021-Cryptographic Failures"},
		SEICert: []string{"MSC02-J"},
	},

	// TLS / certificate validation
	"InsecureTrustManager": {
		CWE:   []string{"CWE-295"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"AllowAllHostnameVerifier": {
		CWE:   []string{"CWE-297"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"OkHttpDisableSslValidation": {
		CWE:   []string{"CWE-295"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"DisableCertificatePinning": {
		CWE:   []string{"CWE-295"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"HardcodedHttpUrl": {
		CWE:   []string{"CWE-319"},
		OWASP: []string{"A02:2021-Cryptographic Failures"},
	},

	// Hardcoded secrets
	"HardcodedSecretKey": {
		CWE:     []string{"CWE-798"},
		OWASP:   []string{"A07:2021-Identification and Authentication Failures"},
		SEICert: []string{"MSC03-J"},
	},
	"HardcodedAwsAccessKey": {
		CWE:   []string{"CWE-798"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"HardcodedBearerToken": {
		CWE:   []string{"CWE-798"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"HardcodedJwt": {
		CWE:   []string{"CWE-798"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"HardcodedGcpServiceAccount": {
		CWE:   []string{"CWE-798"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},
	"GoogleApiKeyInResources": {
		CWE:   []string{"CWE-798"},
		OWASP: []string{"A07:2021-Identification and Authentication Failures"},
	},

	// Sensitive data exposure
	"LogPii": {
		CWE:     []string{"CWE-532"},
		OWASP:   []string{"A09:2021-Security Logging and Monitoring Failures"},
		SEICert: []string{"FIO13-J"},
	},
	"TempFileWorldReadable": {
		CWE:     []string{"CWE-732"},
		OWASP:   []string{"A01:2021-Broken Access Control"},
		SEICert: []string{"FIO01-J"},
	},

	// Android IPC / intent / receiver exposure
	"ImplicitPendingIntent": {
		CWE:   []string{"CWE-927"},
		OWASP: []string{"A01:2021-Broken Access Control"},
	},
	"StartActivityWithUntrustedIntent": {
		CWE:   []string{"CWE-926"},
		OWASP: []string{"A01:2021-Broken Access Control"},
	},
	"UnprotectedDynamicReceiver": {
		CWE:   []string{"CWE-925", "CWE-926"},
		OWASP: []string{"A01:2021-Broken Access Control"},
	},
	"BroadcastReceiverExportedFlagMissing": {
		CWE:   []string{"CWE-926"},
		OWASP: []string{"A05:2021-Security Misconfiguration"},
	},
	"DeepLinkMissingAutoVerify": {
		CWE:   []string{"CWE-940"},
		OWASP: []string{"A05:2021-Security Misconfiguration"},
	},
	"NetworkSecurityConfigDebugOverrides": {
		CWE:   []string{"CWE-489", "CWE-319"},
		OWASP: []string{"A05:2021-Security Misconfiguration"},
	},
}

// applySecurityTaxonomy attaches taxonomy mappings to security-category
// rules in the registry. Called from registry_bootstrap init() after
// all rules have been registered.
func applySecurityTaxonomy() {
	for _, r := range api.Registry {
		taxonomy, ok := securityTaxonomyByRuleID[r.ID]
		if !ok {
			continue
		}
		if r.Security == nil {
			r.Security = taxonomy
		}
	}
}
