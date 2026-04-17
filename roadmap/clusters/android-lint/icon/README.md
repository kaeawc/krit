# Android Lint — Icon Sub-Cluster

Rules that analyze drawable and mipmap resource images. Implemented via the `IconRule` interface and standalone `CheckXxx` functions in `internal/rules/android_icons.go`. Binary image analysis (PNG, GIF, WebP) is handled by `internal/android/convert.go` and `icons.go`.

**Status: 12 shipped, 0 planned**

---

## Shipped Rules

### Registered via IconRule interface (9)

| Rule ID | Brief |
|---|---|
| ConvertToWebp | Large PNG could be smaller as WebP |
| GifUsage | GIF file in resources |
| IconDensities | Missing density variants for icon |
| IconDipSize | Icon dimensions don't match expected DPI ratios |
| IconDuplicates | Same image across densities without scaling |
| IconDuplicatesConfig | Identical icons across configuration folders |
| IconExpectedSize | Launcher icon not at expected size |
| IconMissingDensityFolder | Missing density folder |
| IconNoDpi | Icon in both nodpi and density-specific folder |

### Implemented as CheckXxx functions (3)

These run outside the registered rule pipeline via direct calls in the icon index scan:

| Rule ID | Function | Brief |
|---|---|---|
| IconColors | `CheckIconColors` | Action bar icons should use primarily white/gray colors |
| IconLauncherShape | `CheckIconLauncherShape` | Launcher icon PNG has square corners (should have transparent rounded corners) |
| IconXmlAndPng | `CheckIconXmlAndPng` | Resource exists as both XML vector and raster (PNG/JPG/WebP) format |

---

## Implementation Notes

- **Binary detection**: Animated GIF detection reads the GIF header block count. Animated PNG (APNG) detection scans for the `acTL` chunk before the first `IDAT` chunk. Both live in `internal/android/icons.go`.
- **Autofix**: `ConvertToWebp` is the only icon rule with autofix capability. It invokes the system `cwebp` tool to convert oversized PNGs and rewrites the file path in the resource index. Fix level: `FixCosmetic`.
- **Density expectations**: `IconDipSize` and `IconExpectedSize` use the standard Android density bucket table (ldpi=0.75, mdpi=1.0, hdpi=1.5, xhdpi=2.0, xxhdpi=3.0, xxxhdpi=4.0). Expected launcher sizes are 36/48/72/96/144/192 px.
- **No planned rules**: All 13 canonical AOSP icon rules are covered. `IconExtension`, `IconLocation`, and `IconMixedNinePatch` are tracked in the AOSP compatibility map but have no open implementation gap.
