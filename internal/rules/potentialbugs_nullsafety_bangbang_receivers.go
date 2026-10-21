package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func isViewBindingReceiver(receiver string) bool {
	if strings.HasPrefix(receiver, "_") && !strings.ContainsAny(receiver, "().") {
		return true
	}
	return receiver == "binding" || receiver == "viewBinding" || receiver == "_binding"
}

func isFragmentLifecycleReceiver(receiver string) bool {
	switch receiver {
	case "instance", "INSTANCE",
		"context", "activity", "arguments", "window",
		"dialog", "parentFragment", "serializedData":
		return true
	}
	if strings.HasSuffix(receiver, ".window") {
		if strings.Contains(strings.ToLower(receiver), "dialog") {
			return true
		}
	}
	return false
}

func isAndroidResourceReceiver(receiver string) bool {
	androidGetters := []string{
		"getDrawable(", "getColorStateList(", "getBundleExtra(",
		"getParcelableExtra(", "getParcelableExtraCompat(",
		"getParcelableArrayExtraCompat(", "getParcelableArrayListExtraCompat(",
		"getStringExtra(", "getIntExtra(", "getSystemService",
		"modelClass.cast(", ".cast(",
	}
	for _, g := range androidGetters {
		if strings.Contains(receiver, g) {
			return true
		}
	}
	if strings.HasSuffix(receiver, ".extras") ||
		strings.HasSuffix(receiver, "intent.extras") {
		return true
	}
	return false
}

func isWireProtobufReceiver(receiver string) bool {
	wireDecoders := []string{
		".ADAPTER.decode(", "cursor.requireBlob(", "requireBlob(", "requireNonNullBlob(",
	}
	for _, d := range wireDecoders {
		if strings.Contains(receiver, d) {
			return true
		}
	}
	wireProtoFields := []string{
		".timestamp", ".serverTimestamp", ".sourceDevice", ".sourceServiceId",
		".destination", ".destinationServiceId", ".groupId", ".masterKey",
		".content", ".dataMessage", ".syncMessage", ".sent", ".message",
		".type", ".serverGuid", ".ciphertextHash",
		".amount", ".badge", ".metadata", ".redemption", ".accessControl",
		".start", ".length", ".value", ".address", ".body", ".uri",
		".query", ".recipient", ".singleRecipient",
		".callMessage", ".offer", ".answer", ".hangup", ".busy", ".opaque",
		".fetchLatest", ".messageRequestResponse", ".blocked", ".verified",
		".configuration", ".keys", ".storageService", ".contacts",
		".callEvent", ".callLinkUpdate", ".callLogEvent", ".deleteForMe",
		".storyMessage", ".editMessage", ".giftBadge", ".paymentNotification",
		".inAppPayment", ".uploadSpec", ".backupData", ".credentials",
		".cdn", ".avatar", ".viewOnceOpen", ".outgoingPayment",
		".senderDevice", ".needsReceipt", ".serverReceivedTimestamp",
		".remoteDigest", ".aci", ".pni", ".style", ".receiptCredentialPresentation",
		".paymentMethod", ".failureReason", ".cancellationReason",
		".id", ".data_", ".targetSentTimestamp", ".latestRevisionId",
		".direction", ".conversationId", ".event", ".peekInfo",
		".ringUpdate", ".acknowledgedReceipt", ".observedReceipt",
		".flags", ".delete", ".edit", ".reaction", ".thread", ".groupV2",
		".sticker", ".preview", ".attachments", ".quote",
	}
	for _, field := range wireProtoFields {
		if strings.HasSuffix(receiver, field) {
			return true
		}
	}
	return false
}

func isBundleOrParcelReceiver(receiver string) bool {
	bundleMethods := []string{
		".getString(", ".getStringArray(", ".getStringArrayList(",
		".getInt(", ".getIntArray(", ".getIntegerArrayList(",
		".getLong(", ".getLongArray(",
		".getFloat(", ".getFloatArray(",
		".getDouble(", ".getDoubleArray(",
		".getBoolean(", ".getBooleanArray(",
		".getByte(", ".getByteArray(",
		".getChar(", ".getCharArray(",
		".getShort(", ".getShortArray(",
		".getParcelable(", ".getParcelableArray(", ".getParcelableArrayList(",
		".getParcelableCompat(", ".getParcelableArrayCompat(",
		".getParcelableArrayListCompat(",
		".getSerializable(", ".getSerializableCompat(",
		".getBundle(", ".getCharSequence(", ".getCharSequenceArray(",
		".getCharSequenceArrayList(",
	}
	for _, m := range bundleMethods {
		if strings.Contains(receiver, m) {
			return true
		}
	}
	parcelMethods := []string{
		".readString(", ".readStringArray(", ".readStringList(",
		".readInt(", ".readLong(", ".readFloat(", ".readDouble(",
		".readByte(", ".readByteArray(", ".readBundle(",
		".readParcelable(", ".readParcelableArray(", ".readParcelableList(",
		".readParcelableCompat(", ".readParcelableArrayCompat(",
		".readSerializable(",
		".readSerializableCompat(",
	}
	for _, m := range parcelMethods {
		if strings.Contains(receiver, m) {
			return true
		}
	}
	return false
}

func isKspReceiver(receiver string, file *scanner.File) bool {
	if fileImportsKsp(file) && strings.HasSuffix(receiver, ".qualifiedName") {
		return true
	}
	if fileImportsKsp(file) && receiver == "creatorOrConstructor" {
		return true
	}
	return false
}

func isUnqualifiedWireProtoField(receiver string) bool {
	if receiver == "" || strings.ContainsAny(receiver, ".()[]") {
		return false
	}
	switch receiver {
	case "accessControl", "address", "amount", "attachments", "badge", "body",
		"callEvent", "callLinkUpdate", "callLogEvent", "callMessage", "cdn",
		"ciphertextHash", "configuration", "contacts", "content", "dataMessage",
		"data_", "delete", "deleteForMe", "destination", "destinationServiceId",
		"direction", "edit", "editMessage", "event", "failureReason", "fetchLatest",
		"flags", "giftBadge", "groupId", "groupV2", "groupChange", "hangup",
		"id", "inAppPayment", "keys", "latestRevisionId", "length", "masterKey",
		"message", "messageRequestResponse", "metadata", "needsReceipt", "opaque",
		"outgoingPayment", "paymentMethod", "paymentNotification", "preview",
		"query", "reaction", "receiptCredentialPresentation", "recipient",
		"redemption", "remoteDigest", "ringUpdate", "senderDevice", "sent",
		"serverGuid", "serverReceivedTimestamp", "serverTimestamp", "sourceDevice",
		"sourceServiceId", "start", "sticker", "storageService", "storyMessage",
		"syncMessage", "targetSentTimestamp", "thread", "timestamp", "type",
		"uploadSpec", "uri", "value", "verified", "viewOnceOpen":
		return true
	default:
		return false
	}
}

func isInsideWireProtoReceiverExtensionFlat(file *scanner.File, receiverIdx uint32) bool {
	if file == nil || receiverIdx == 0 {
		return false
	}
	if fn, ok := flatEnclosingFunction(file, receiverIdx); ok {
		typeName, _ := flatFunctionReceiverTypeInfo(file, fn)
		return isWireProtoReceiverTypeName(typeName)
	}
	prop, ok := flatEnclosingAncestor(file, receiverIdx, "property_declaration")
	if !ok {
		return false
	}
	typeName := flatPropertyReceiverTypeName(file, prop)
	return isWireProtoReceiverTypeName(typeName)
}

func isWireProtoReceiverTypeName(typeName string) bool {
	switch typeName {
	case "DataMessage", "GroupContextV2", "SyncMessage", "Content", "Envelope",
		"PendingOneTimeDonation", "FiatValue", "DecimalValue", "DecryptedGroup":
		return true
	default:
		return false
	}
}

func flatPropertyReceiverTypeName(file *scanner.File, prop uint32) string {
	if file == nil || prop == 0 || file.FlatType(prop) != "property_declaration" {
		return ""
	}
	var name string
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "nullable_type", "user_type", "type_identifier":
			if name == "" {
				name = flatLastIdentifierInNode(file, child)
			}
		case ".":
			return name
		case "property_delegate", "property_declaration_body", "=", "getter":
			return ""
		}
	}
	return ""
}

// isCompilerLookupReceiver reports compiler-plugin symbol lookups where `!!`
// is the conventional "this lookup must exist" assertion. This keeps the rule
// focused on application code while avoiding noisy compiler/IR codegen paths.
func isCompilerLookupReceiver(receiver string) bool {
	if strings.Contains(receiver, "referenceClass(") ||
		strings.Contains(receiver, "primaryConstructor") ||
		strings.Contains(receiver, "classFqName") ||
		strings.Contains(receiver, "getter") ||
		strings.Contains(receiver, "resolveKSClassDeclaration(") ||
		receiver == "classId" || strings.HasSuffix(receiver, ".classId") ||
		receiver == "creatorOrConstructor" || strings.HasSuffix(receiver, ".creatorOrConstructor") ||
		strings.Contains(receiver, "companionObject()") {
		return true
	}
	return isCompilerPluginInvariantReceiver(receiver)
}

func isCompilerPluginInvariantReceiver(receiver string) bool {
	if receiver == "" {
		return false
	}
	for _, part := range []string{
		"constArgumentOfTypeAt<",
		"CompilerMessageLocationWithRange.create(",
		"findRequiredConstructor(",
		"getAnnotation(",
		"getRequiredAnnotationString()",
		"scopeOrNull()",
		"typeArguments.single()",
		"RequiredMapKeyAnnotation()",
		"findRequiredMapValueType()",
		"requiredClassId",
	} {
		if strings.Contains(receiver, part) {
			return true
		}
	}
	for _, suffix := range []string{
		".backingField",
		".callee",
		".dispatchReceiverParameter",
		".extensionReceiverParameterCompat",
		".factoryTargetMetadata",
		".generatedGraphMetadata",
		".graphParam",
		".ir",
		".irElement",
		".irProperty",
		".dependencyGraph",
		".packageFqName",
		".receiverParameterSymbol",
		".scope",
		".switchingId",
		".targetConstructor",
		".thisReceiver",
		".typeOrNull",
	} {
		if strings.HasSuffix(receiver, suffix) {
			return true
		}
	}
	switch receiver {
	case "backingField", "containerParameter", "diagnostic", "dispatchReceiverParameter",
		"functionReceiver", "generatedGraphMetadata", "innerRequiredClassId", "injectorMetadata",
		"mapKey", "parentClassOrNull", "requiredClassId", "requiredType", "sizeSymbol",
		"targetConstructor", "thisReceiver":
		return true
	default:
		return false
	}
}
