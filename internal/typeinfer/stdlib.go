package typeinfer

// StdlibMethod describes a known stdlib method's return type.
type StdlibMethod struct {
	ReturnType         *ResolvedType
	Nullable           bool
	ReturnTypeArgIndex int // -1 = use fixed ReturnType; 0+ = use receiver's TypeArgs[i]
}

// stdlibType is a shorthand to build a ResolvedType for the stdlib table.
func stdlibType(name string, kind TypeKind) *ResolvedType {
	fqn := ""
	if f, ok := PrimitiveTypes[name]; ok {
		fqn = f
	} else if f, ok := KotlinStdlibTypes[name]; ok {
		fqn = f
	}
	return &ResolvedType{Name: name, FQN: fqn, Kind: kind}
}

func stdlibPrimitive(name string) *ResolvedType {
	return stdlibType(name, TypePrimitive)
}

func stdlibClass(name string) *ResolvedType {
	return stdlibType(name, TypeClass)
}

func stdlibUnit() *ResolvedType {
	return &ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: TypeUnit}
}

// StdlibMethods maps "ReceiverType.methodName" -> return type info.
// The receiver type uses simple names (String, List, Map, etc.).
// For collection types, List/Collection/Iterable/MutableList share entries.
var StdlibMethods map[string]*StdlibMethod

func init() {
	StdlibMethods = make(map[string]*StdlibMethod)

	// -------------------------------------------------------------------------
	// kotlin.String methods
	// -------------------------------------------------------------------------
	stringMethods := map[string]*StdlibMethod{
		"toInt":        {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"toLong":       {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"toFloat":      {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"toDouble":     {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"toByte":       {ReturnType: stdlibPrimitive("Byte"), ReturnTypeArgIndex: -1},
		"toShort":      {ReturnType: stdlibPrimitive("Short"), ReturnTypeArgIndex: -1},
		"toIntOrNull":  {ReturnType: stdlibPrimitive("Int"), Nullable: true, ReturnTypeArgIndex: -1},
		"toLongOrNull": {ReturnType: stdlibPrimitive("Long"), Nullable: true, ReturnTypeArgIndex: -1},
		"lowercase":    {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"uppercase":    {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"trim":         {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"trimIndent":   {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"trimMargin":   {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"replace":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"reversed":     {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"length":       {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"isEmpty":      {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isBlank":      {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isNotEmpty":   {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isNotBlank":   {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"startsWith":   {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"endsWith":     {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"contains":     {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"toByteArray":  {ReturnType: stdlibClass("ByteArray"), ReturnTypeArgIndex: -1},
		"split":        {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"toCharArray":  {ReturnType: stdlibClass("CharArray"), ReturnTypeArgIndex: -1},
		"toRegex":      {ReturnType: stdlibClass("Regex"), ReturnTypeArgIndex: -1},
	}
	for method, info := range stringMethods {
		StdlibMethods["String."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.collections.List / Collection / Iterable methods
	// -------------------------------------------------------------------------
	collectionMethods := map[string]*StdlibMethod{
		"map":                {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"filter":             {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"filterNot":          {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"filterNotNull":      {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"flatMap":            {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"first":              {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"last":               {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"single":             {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"firstOrNull":        {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"lastOrNull":         {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"singleOrNull":       {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"find":               {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"findLast":           {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"any":                {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"all":                {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"none":               {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"count":              {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"size":               {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"isEmpty":            {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isNotEmpty":         {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"toList":             {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"toMutableList":      {ReturnType: stdlibClass("MutableList"), ReturnTypeArgIndex: -1},
		"toSet":              {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"sorted":             {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"sortedBy":           {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"sortedWith":         {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"sortedDescending":   {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"sortedByDescending": {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"reversed":           {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"distinct":           {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"distinctBy":         {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"take":               {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"drop":               {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"takeWhile":          {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"dropWhile":          {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"takeLast":           {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"dropLast":           {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"zip":                {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"associate":          {ReturnType: stdlibClass("Map"), ReturnTypeArgIndex: -1},
		"groupBy":            {ReturnType: stdlibClass("Map"), ReturnTypeArgIndex: -1},
		"joinToString":       {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"forEach":            {ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1},
		"sumOf":              {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
	}
	for _, receiver := range []string{"List", "Collection", "Iterable", "MutableList", "ArrayList", "Set", "MutableSet", "HashSet", "LinkedHashSet"} {
		for method, info := range collectionMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// -------------------------------------------------------------------------
	// kotlin.collections.Map methods
	// -------------------------------------------------------------------------
	mapMethods := map[string]*StdlibMethod{
		"get":           {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 1},
		"getValue":      {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 1},
		"getOrDefault":  {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 1},
		"getOrElse":     {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 1},
		"keys":          {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"values":        {ReturnType: stdlibClass("Collection"), ReturnTypeArgIndex: -1},
		"entries":       {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"containsKey":   {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"containsValue": {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isEmpty":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"size":          {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
	}
	for _, receiver := range []string{"Map", "MutableMap", "HashMap", "LinkedHashMap"} {
		for method, info := range mapMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// -------------------------------------------------------------------------
	// kotlin.sequences.Sequence methods
	// -------------------------------------------------------------------------
	seqMethods := map[string]*StdlibMethod{
		"map":                {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"filter":             {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"flatMap":            {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"sorted":             {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"sortedBy":           {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"sortedWith":         {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"sortedDescending":   {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"sortedByDescending": {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"distinct":           {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"distinctBy":         {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"drop":               {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"dropWhile":          {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"take":               {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"takeWhile":          {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"zip":                {ReturnType: stdlibClass("Sequence"), ReturnTypeArgIndex: -1},
		"toList":             {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"toSet":              {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"first":              {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"firstOrNull":        {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"count":              {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"any":                {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"all":                {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"none":               {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
	}
	for method, info := range seqMethods {
		StdlibMethods["Sequence."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Int methods
	// -------------------------------------------------------------------------
	intMethods := map[string]*StdlibMethod{
		"toString":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"toLong":        {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"toFloat":       {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"toDouble":      {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"toShort":       {ReturnType: stdlibPrimitive("Short"), ReturnTypeArgIndex: -1},
		"toByte":        {ReturnType: stdlibPrimitive("Byte"), ReturnTypeArgIndex: -1},
		"toChar":        {ReturnType: stdlibPrimitive("Char"), ReturnTypeArgIndex: -1},
		"coerceIn":      {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"coerceAtLeast": {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"coerceAtMost":  {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"compareTo":     {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"plus":          {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"minus":         {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"times":         {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"div":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"rem":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"mod":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"inc":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"dec":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"unaryMinus":    {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"unaryPlus":     {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"shl":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"shr":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"ushr":          {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"and":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"or":            {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"xor":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"inv":           {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
	}
	for method, info := range intMethods {
		StdlibMethods["Int."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Long methods
	// -------------------------------------------------------------------------
	longMethods := map[string]*StdlibMethod{
		"toString":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"toInt":         {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"toFloat":       {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"toDouble":      {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"toShort":       {ReturnType: stdlibPrimitive("Short"), ReturnTypeArgIndex: -1},
		"toByte":        {ReturnType: stdlibPrimitive("Byte"), ReturnTypeArgIndex: -1},
		"toChar":        {ReturnType: stdlibPrimitive("Char"), ReturnTypeArgIndex: -1},
		"coerceIn":      {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"coerceAtLeast": {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"coerceAtMost":  {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"compareTo":     {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"plus":          {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"minus":         {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"times":         {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"div":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"rem":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"mod":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"inc":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"dec":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"unaryMinus":    {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"unaryPlus":     {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"shl":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"shr":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"ushr":          {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"and":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"or":            {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"xor":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"inv":           {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
	}
	for method, info := range longMethods {
		StdlibMethods["Long."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Double methods
	// -------------------------------------------------------------------------
	doubleMethods := map[string]*StdlibMethod{
		"toString":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"toInt":         {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"toLong":        {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"toFloat":       {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"toByte":        {ReturnType: stdlibPrimitive("Byte"), ReturnTypeArgIndex: -1},
		"toShort":       {ReturnType: stdlibPrimitive("Short"), ReturnTypeArgIndex: -1},
		"coerceIn":      {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"coerceAtLeast": {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"coerceAtMost":  {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"compareTo":     {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"plus":          {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"minus":         {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"times":         {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"div":           {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"rem":           {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"inc":           {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"dec":           {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"unaryMinus":    {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"unaryPlus":     {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"isNaN":         {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isInfinite":    {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isFinite":      {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
	}
	for method, info := range doubleMethods {
		StdlibMethods["Double."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Float methods
	// -------------------------------------------------------------------------
	floatMethods := map[string]*StdlibMethod{
		"toString":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"toInt":         {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"toLong":        {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"toDouble":      {ReturnType: stdlibPrimitive("Double"), ReturnTypeArgIndex: -1},
		"toByte":        {ReturnType: stdlibPrimitive("Byte"), ReturnTypeArgIndex: -1},
		"toShort":       {ReturnType: stdlibPrimitive("Short"), ReturnTypeArgIndex: -1},
		"coerceIn":      {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"coerceAtLeast": {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"coerceAtMost":  {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"compareTo":     {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"plus":          {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"minus":         {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"times":         {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"div":           {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"rem":           {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"inc":           {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"dec":           {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"unaryMinus":    {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"unaryPlus":     {ReturnType: stdlibPrimitive("Float"), ReturnTypeArgIndex: -1},
		"isNaN":         {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isInfinite":    {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isFinite":      {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
	}
	for method, info := range floatMethods {
		StdlibMethods["Float."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Boolean methods
	// -------------------------------------------------------------------------
	boolMethods := map[string]*StdlibMethod{
		"toString":  {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"compareTo": {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"not":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"and":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"or":        {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"xor":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
	}
	for method, info := range boolMethods {
		StdlibMethods["Boolean."+method] = info
	}

	// -------------------------------------------------------------------------
	// Android Context methods
	// -------------------------------------------------------------------------
	StdlibMethods["Context.getString"] = &StdlibMethod{ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1}
	StdlibMethods["Context.getResources"] = &StdlibMethod{ReturnType: stdlibClass("Resources"), ReturnTypeArgIndex: -1}
	StdlibMethods["Context.getSystemService"] = &StdlibMethod{ReturnType: stdlibClass("Any"), Nullable: true, ReturnTypeArgIndex: -1}
	StdlibMethods["Context.getPackageName"] = &StdlibMethod{ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1}
	StdlibMethods["Context.getApplicationContext"] = &StdlibMethod{ReturnType: stdlibClass("Context"), ReturnTypeArgIndex: -1}
	StdlibMethods["Context.startActivity"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}

	// -------------------------------------------------------------------------
	// java.io.File methods
	// -------------------------------------------------------------------------
	fileMethods := map[string]*StdlibMethod{
		"exists":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isFile":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"isDirectory":  {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"readText":     {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"readLines":    {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"writeText":    {ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1},
		"delete":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"mkdirs":       {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
		"getName":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"getPath":      {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
		"getParent":    {ReturnType: stdlibPrimitive("String"), Nullable: true, ReturnTypeArgIndex: -1},
		"length":       {ReturnType: stdlibPrimitive("Long"), ReturnTypeArgIndex: -1},
		"listFiles":    {ReturnType: stdlibClass("Array"), Nullable: true, ReturnTypeArgIndex: -1},
		"absolutePath": {ReturnType: stdlibPrimitive("String"), ReturnTypeArgIndex: -1},
	}
	for method, info := range fileMethods {
		StdlibMethods["File."+method] = info
	}

	// -------------------------------------------------------------------------
	// Set-specific methods (not shared with List)
	// -------------------------------------------------------------------------
	setSpecificMethods := map[string]*StdlibMethod{
		"intersect":    {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"union":        {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"subtract":     {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"plus":         {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"minus":        {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"toMutableSet": {ReturnType: stdlibClass("MutableSet"), ReturnTypeArgIndex: -1},
		"contains":     {ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1},
	}
	for _, receiver := range []string{"Set", "MutableSet", "HashSet", "LinkedHashSet"} {
		for method, info := range setSpecificMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// -------------------------------------------------------------------------
	// MutableCollection mutation methods
	// -------------------------------------------------------------------------
	// MutableList
	for _, receiver := range []string{"MutableList", "ArrayList"} {
		StdlibMethods[receiver+".add"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".remove"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".removeAt"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0}
		StdlibMethods[receiver+".set"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0}
		StdlibMethods[receiver+".clear"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".addAll"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".removeAll"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".retainAll"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".subList"] = &StdlibMethod{ReturnType: stdlibClass("MutableList"), ReturnTypeArgIndex: -1}
	}

	// MutableSet
	for _, receiver := range []string{"MutableSet", "HashSet", "LinkedHashSet"} {
		StdlibMethods[receiver+".add"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".remove"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".clear"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".addAll"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".removeAll"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".retainAll"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
	}

	// MutableMap
	for _, receiver := range []string{"MutableMap", "HashMap", "LinkedHashMap"} {
		StdlibMethods[receiver+".put"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 1}
		StdlibMethods[receiver+".remove"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 1}
		StdlibMethods[receiver+".clear"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".putAll"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
		StdlibMethods[receiver+".getOrPut"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 1}
		StdlibMethods[receiver+".putIfAbsent"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 1}
	}

	// -------------------------------------------------------------------------
	// kotlinx.coroutines.flow.Flow methods
	// -------------------------------------------------------------------------
	flowMethods := map[string]*StdlibMethod{
		"map":                  {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"filter":               {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"filterNot":            {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"filterNotNull":        {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"flatMapConcat":        {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"flatMapMerge":         {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"flatMapLatest":        {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"transform":            {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"onEach":               {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"onStart":              {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"onCompletion":         {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"catch":                {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"retry":                {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"retryWhen":            {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"take":                 {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"takeWhile":            {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"drop":                 {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"dropWhile":            {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"distinctUntilChanged": {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"debounce":             {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"sample":               {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"conflate":             {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"buffer":               {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"flowOn":               {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"combine":              {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"zip":                  {ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1},
		"collect":              {ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1},
		"collectLatest":        {ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1},
		"first":                {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"firstOrNull":          {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"single":               {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"singleOrNull":         {ReturnType: stdlibType("Any", TypeClass), Nullable: true, ReturnTypeArgIndex: 0},
		"toList":               {ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1},
		"toSet":                {ReturnType: stdlibClass("Set"), ReturnTypeArgIndex: -1},
		"count":                {ReturnType: stdlibPrimitive("Int"), ReturnTypeArgIndex: -1},
		"fold":                 {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: -1},
		"reduce":               {ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0},
		"stateIn":              {ReturnType: stdlibClass("StateFlow"), ReturnTypeArgIndex: -1},
		"shareIn":              {ReturnType: stdlibClass("SharedFlow"), ReturnTypeArgIndex: -1},
		"launchIn":             {ReturnType: stdlibClass("Job"), ReturnTypeArgIndex: -1},
	}
	for _, receiver := range []string{"Flow", "StateFlow", "SharedFlow", "MutableStateFlow", "MutableSharedFlow"} {
		for method, info := range flowMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// StateFlow/SharedFlow specific properties
	StdlibMethods["StateFlow.value"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0}
	StdlibMethods["MutableStateFlow.value"] = &StdlibMethod{ReturnType: stdlibType("Any", TypeClass), ReturnTypeArgIndex: 0}
	StdlibMethods["SharedFlow.replayCache"] = &StdlibMethod{ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1}
	StdlibMethods["MutableSharedFlow.replayCache"] = &StdlibMethod{ReturnType: stdlibClass("List"), ReturnTypeArgIndex: -1}
	StdlibMethods["MutableSharedFlow.emit"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
	StdlibMethods["MutableSharedFlow.tryEmit"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}
	StdlibMethods["MutableStateFlow.compareAndSet"] = &StdlibMethod{ReturnType: stdlibPrimitive("Boolean"), ReturnTypeArgIndex: -1}

	// -------------------------------------------------------------------------
	// kotlinx.coroutines methods
	// -------------------------------------------------------------------------
	StdlibMethods["CoroutineScope.launch"] = &StdlibMethod{ReturnType: stdlibClass("Job"), ReturnTypeArgIndex: -1}
	StdlibMethods["CoroutineScope.async"] = &StdlibMethod{ReturnType: stdlibClass("Deferred"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.withContext"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.delay"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
	StdlibMethods["_.runBlocking"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.coroutineScope"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.supervisorScope"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.yield"] = &StdlibMethod{ReturnType: stdlibUnit(), ReturnTypeArgIndex: -1}
	StdlibMethods["_.withTimeout"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.withTimeoutOrNull"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, Nullable: true, ReturnTypeArgIndex: -1}
	StdlibMethods["_.flow"] = &StdlibMethod{ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.flowOf"] = &StdlibMethod{ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.channelFlow"] = &StdlibMethod{ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.callbackFlow"] = &StdlibMethod{ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.emptyFlow"] = &StdlibMethod{ReturnType: stdlibClass("Flow"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.MutableStateFlow"] = &StdlibMethod{ReturnType: stdlibClass("MutableStateFlow"), ReturnTypeArgIndex: -1}
	StdlibMethods["_.MutableSharedFlow"] = &StdlibMethod{ReturnType: stdlibClass("MutableSharedFlow"), ReturnTypeArgIndex: -1}

	// -------------------------------------------------------------------------
	// Kotlin scope functions
	// -------------------------------------------------------------------------
	// let/run return the lambda result (unknown without full inference)
	// apply/also return the receiver — handled specially in resolve.go
	// with is a top-level function that returns the lambda result
	StdlibMethods["_.let"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.run"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.with"] = &StdlibMethod{ReturnType: &ResolvedType{Kind: TypeUnknown}, ReturnTypeArgIndex: -1}

	// -------------------------------------------------------------------------
	// Nothing-returning top-level functions
	// -------------------------------------------------------------------------
	StdlibMethods["_.TODO"] = &StdlibMethod{ReturnType: &ResolvedType{Name: "Nothing", FQN: "kotlin.Nothing", Kind: TypeNothing}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.error"] = &StdlibMethod{ReturnType: &ResolvedType{Name: "Nothing", FQN: "kotlin.Nothing", Kind: TypeNothing}, ReturnTypeArgIndex: -1}
}

// LookupStdlibMethod looks up a stdlib method by receiver type name and method name.
// It tries the exact receiver first, then falls back to common supertype aliases.
func LookupStdlibMethod(receiverType, methodName string) *StdlibMethod {
	key := receiverType + "." + methodName
	if m, ok := StdlibMethods[key]; ok {
		return m
	}
	// Try top-level function lookup (receiver-less)
	topKey := "_." + methodName
	if m, ok := StdlibMethods[topKey]; ok {
		return m
	}
	return nil
}

// ---------------------------------------------------------------------------
// Known interface implementors (framework types tree-sitter can't resolve)
// ---------------------------------------------------------------------------

// KnownInterfaces maps interface FQN -> known implementors.
var KnownInterfaces = map[string][]string{
	"java.io.Serializable": {
		"java.lang.Number", "java.lang.String", "java.lang.Enum",
		"java.util.ArrayList", "java.util.HashMap", "java.util.HashSet",
		"java.util.LinkedList", "java.util.TreeMap", "java.util.TreeSet",
		"kotlin.Pair", "kotlin.Triple",
		"java.util.Date", "java.util.Calendar",
		"java.math.BigDecimal", "java.math.BigInteger",
		"java.io.File", "java.net.URL", "java.net.URI",
	},
	"java.io.Closeable": {
		"java.io.InputStream", "java.io.OutputStream",
		"java.io.FileInputStream", "java.io.FileOutputStream",
		"java.io.BufferedReader", "java.io.BufferedWriter",
		"java.io.InputStreamReader", "java.io.OutputStreamWriter",
		"java.io.PrintWriter", "java.io.PrintStream",
		"java.net.Socket", "java.net.ServerSocket",
		"java.sql.Connection", "java.sql.Statement", "java.sql.ResultSet",
		"java.util.Scanner",
	},
	"android.os.Parcelable": {
		"android.os.Bundle", "android.content.Intent",
		"android.net.Uri", "android.graphics.Bitmap",
	},
}

// ---------------------------------------------------------------------------
// Known class hierarchies (Android/Java framework types)
// ---------------------------------------------------------------------------

// KnownClassHierarchy maps FQN -> ordered list of supertypes (nearest first).
var KnownClassHierarchy = map[string][]string{
	// Android components
	"android.app.Activity":                     {"android.content.ContextWrapper", "android.content.Context"},
	"androidx.fragment.app.FragmentActivity":   {"android.app.Activity", "android.content.ContextWrapper", "android.content.Context"},
	"androidx.appcompat.app.AppCompatActivity": {"androidx.fragment.app.FragmentActivity", "android.app.Activity"},
	"android.app.Application":                  {"android.content.ContextWrapper", "android.content.Context"},
	"android.app.Service":                      {"android.content.ContextWrapper", "android.content.Context"},
	"android.content.BroadcastReceiver":        {},
	"android.content.ContentProvider":          {},
	// Views
	"android.view.ViewGroup":        {"android.view.View"},
	"android.widget.LinearLayout":   {"android.view.ViewGroup", "android.view.View"},
	"android.widget.FrameLayout":    {"android.view.ViewGroup", "android.view.View"},
	"android.widget.RelativeLayout": {"android.view.ViewGroup", "android.view.View"},
	"android.widget.TextView":       {"android.view.View"},
	"android.widget.ImageView":      {"android.view.View"},
	"android.widget.Button":         {"android.widget.TextView", "android.view.View"},
	// Java
	"java.lang.Exception":        {"java.lang.Throwable"},
	"java.lang.RuntimeException": {"java.lang.Exception", "java.lang.Throwable"},
}

// simpleNameOf extracts the simple class name from a fully-qualified name.
func simpleNameOf(fqn string) string {
	for i := len(fqn) - 1; i >= 0; i-- {
		if fqn[i] == '.' {
			return fqn[i+1:]
		}
	}
	return fqn
}

// ImplementsInterface checks whether a given FQN is a known implementor of
// the specified interface FQN.
func ImplementsInterface(typeFQN, interfaceFQN string) bool {
	implementors, ok := KnownInterfaces[interfaceFQN]
	if !ok {
		return false
	}
	for _, impl := range implementors {
		if impl == typeFQN {
			return true
		}
	}
	return false
}

// IsKnownSubtype checks whether typeFQN is a known subtype of supertypeFQN
// using KnownClassHierarchy (transitive) and KnownInterfaces.
func IsKnownSubtype(typeFQN, supertypeFQN string) bool {
	if typeFQN == supertypeFQN {
		return true
	}
	// Check class hierarchy (transitive)
	if supers, ok := KnownClassHierarchy[typeFQN]; ok {
		for _, s := range supers {
			if s == supertypeFQN {
				return true
			}
		}
	}
	// Check interface implementors
	if ImplementsInterface(typeFQN, supertypeFQN) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Exception hierarchy
// ---------------------------------------------------------------------------

// ExceptionAncestors maps exception class name -> ordered list of supertypes.
var ExceptionAncestors = map[string][]string{
	// Root types
	"Throwable":        {},
	"Exception":        {"Throwable"},
	"RuntimeException": {"Exception", "Throwable"},
	"Error":            {"Throwable"},

	// Checked Exception subtypes
	"IOException":                  {"Exception", "Throwable"},
	"FileNotFoundException":        {"IOException", "Exception", "Throwable"},
	"InterruptedException":         {"Exception", "Throwable"},
	"TimeoutException":             {"Exception", "Throwable"},
	"ParseException":               {"Exception", "Throwable"},
	"CloneNotSupportedException":   {"Exception", "Throwable"},
	"ReflectiveOperationException": {"Exception", "Throwable"},
	"ClassNotFoundException":       {"ReflectiveOperationException", "Exception", "Throwable"},
	"NoSuchMethodException":        {"ReflectiveOperationException", "Exception", "Throwable"},
	"NoSuchFieldException":         {"ReflectiveOperationException", "Exception", "Throwable"},
	"URISyntaxException":           {"Exception", "Throwable"},
	"GeneralSecurityException":     {"Exception", "Throwable"},
	"NoSuchAlgorithmException":     {"GeneralSecurityException", "Exception", "Throwable"},
	"InvalidKeyException":          {"GeneralSecurityException", "Exception", "Throwable"},
	"CertificateException":         {"GeneralSecurityException", "Exception", "Throwable"},

	// IOException subtypes
	"SocketException":        {"IOException", "Exception", "Throwable"},
	"ConnectException":       {"SocketException", "IOException", "Exception", "Throwable"},
	"SocketTimeoutException": {"SocketException", "IOException", "Exception", "Throwable"},
	"MalformedURLException":  {"IOException", "Exception", "Throwable"},
	"UnknownHostException":   {"IOException", "Exception", "Throwable"},
	"EOFException":           {"IOException", "Exception", "Throwable"},
	"SSLException":           {"IOException", "Exception", "Throwable"},
	"SSLHandshakeException":  {"SSLException", "IOException", "Exception", "Throwable"},

	// RuntimeException subtypes
	"NullPointerException":            {"RuntimeException", "Exception", "Throwable"},
	"IllegalArgumentException":        {"RuntimeException", "Exception", "Throwable"},
	"IllegalStateException":           {"RuntimeException", "Exception", "Throwable"},
	"CancellationException":           {"IllegalStateException", "RuntimeException", "Exception", "Throwable"},
	"NumberFormatException":           {"IllegalArgumentException", "RuntimeException", "Exception", "Throwable"},
	"IndexOutOfBoundsException":       {"RuntimeException", "Exception", "Throwable"},
	"ArrayIndexOutOfBoundsException":  {"IndexOutOfBoundsException", "RuntimeException", "Exception", "Throwable"},
	"StringIndexOutOfBoundsException": {"IndexOutOfBoundsException", "RuntimeException", "Exception", "Throwable"},
	"ClassCastException":              {"RuntimeException", "Exception", "Throwable"},
	"TypeCastException":               {"ClassCastException", "RuntimeException", "Exception", "Throwable"},
	"UnsupportedOperationException":   {"RuntimeException", "Exception", "Throwable"},
	"ConcurrentModificationException": {"RuntimeException", "Exception", "Throwable"},
	"NoSuchElementException":          {"RuntimeException", "Exception", "Throwable"},
	"ArithmeticException":             {"RuntimeException", "Exception", "Throwable"},
	"SecurityException":               {"RuntimeException", "Exception", "Throwable"},
	"NegativeArraySizeException":      {"RuntimeException", "Exception", "Throwable"},

	// Kotlin-specific
	"KotlinNullPointerException":           {"NullPointerException", "RuntimeException", "Exception", "Throwable"},
	"UninitializedPropertyAccessException": {"RuntimeException", "Exception", "Throwable"},

	// Error subtypes
	"VirtualMachineError":         {"Error", "Throwable"},
	"OutOfMemoryError":            {"VirtualMachineError", "Error", "Throwable"},
	"StackOverflowError":          {"VirtualMachineError", "Error", "Throwable"},
	"LinkageError":                {"Error", "Throwable"},
	"ExceptionInInitializerError": {"LinkageError", "Error", "Throwable"},
	"NoClassDefFoundError":        {"LinkageError", "Error", "Throwable"},
	"NotImplementedError":         {"Error", "Throwable"},
	"AssertionError":              {"Error", "Throwable"},

	// kotlinx.coroutines
	"TimeoutCancellationException": {"CancellationException", "IllegalStateException", "RuntimeException", "Exception", "Throwable"},

	// Android-specific
	"AndroidException":          {"Exception", "Throwable"},
	"ActivityNotFoundException": {"RuntimeException", "Exception", "Throwable"},
	"RemoteException":           {"AndroidException", "Exception", "Throwable"},
	"DeadObjectException":       {"RemoteException", "AndroidException", "Exception", "Throwable"},
	"JSONException":             {"Exception", "Throwable"},
	"SQLiteException":           {"RuntimeException", "Exception", "Throwable"},
	"SQLiteConstraintException": {"SQLiteException", "RuntimeException", "Exception", "Throwable"},
	"BadParcelableException":    {"RuntimeException", "Exception", "Throwable"},
}

// KnownValueTypes lists types where referential equality (===) is almost
// certainly wrong and structural equality (==) should be used instead.
// Both Kotlin and Java FQNs are included so the lookup works regardless of
// whether the resolver produced a simple name or a fully-qualified name.
var KnownValueTypes = map[string]bool{
	// Kotlin primitives & wrappers
	"String": true, "kotlin.String": true,
	"Int": true, "kotlin.Int": true,
	"Long": true, "kotlin.Long": true,
	"Double": true, "kotlin.Double": true,
	"Float": true, "kotlin.Float": true,
	"Boolean": true, "kotlin.Boolean": true,
	"Byte": true, "kotlin.Byte": true,
	"Short": true, "kotlin.Short": true,
	"Char": true, "kotlin.Char": true,
	"UInt": true, "kotlin.UInt": true,
	"ULong": true, "kotlin.ULong": true,
	"UByte": true, "kotlin.UByte": true,
	"UShort": true, "kotlin.UShort": true,
	// Kotlin data holders
	"Pair": true, "kotlin.Pair": true,
	"Triple": true, "kotlin.Triple": true,
	// Java boxed types
	"java.lang.String": true, "java.lang.Integer": true, "java.lang.Long": true,
	"java.lang.Double": true, "java.lang.Float": true, "java.lang.Boolean": true,
	"java.lang.Byte": true, "java.lang.Short": true, "java.lang.Character": true,
	// Java math types
	"BigDecimal": true, "java.math.BigDecimal": true,
	"BigInteger": true, "java.math.BigInteger": true,
}

// IsKnownValueType returns true if the resolved type (by Name or FQN) is a
// known value type where structural equality should be preferred.
func IsKnownValueType(rt *ResolvedType) bool {
	if rt == nil || rt.Kind == TypeUnknown {
		return false
	}
	if KnownValueTypes[rt.Name] {
		return true
	}
	if KnownValueTypes[rt.FQN] {
		return true
	}
	return false
}

// IsSubtypeOfException checks if exceptionA is a subtype of exceptionB.
func IsSubtypeOfException(a, b string) bool {
	if a == b {
		return true
	}
	ancestors, ok := ExceptionAncestors[a]
	if !ok {
		return false
	}
	for _, anc := range ancestors {
		if anc == b {
			return true
		}
	}
	return false
}
