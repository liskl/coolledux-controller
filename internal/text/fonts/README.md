# Bitmap Fonts

Font bin files are extracted from the CoolLED 1248 Android APK and **not committed** to this repo (see `.gitignore`). They must be present before `go build`, because `//go:embed` requires the file at compile time.

## Extracting from the APK

With the APK present at `references/coolled-1248.apk`:

```bash
./scripts/extract-fonts.sh
```

Or manually:

```bash
unzip -j -o references/coolled-1248.apk \
    'assets/flutter_assets/assets/coolledux/font_library/unicode_16_bold' \
    -d internal/text/fonts/
mv internal/text/fonts/unicode_16_bold internal/text/fonts/unicode_16_bold.bin
```

## Format

| File | Bytes | Description |
|------|-------|-------------|
| `unicode_16_bold.bin` | 2,097,152 | 65,536 glyphs × 32 bytes. 16-row bold font. |

Each glyph is 16 columns × 16 rows, 1 bit per pixel, **column-major**. Two bytes per column: the first byte is the top 8 pixels (MSB = row 0), the second byte is the bottom 8 pixels. Glyphs are indexed by Unicode code point (U+0000..U+FFFF); supplementary planes are not present.

Many glyphs use only the left 6-8 columns; trailing empty columns are padding that the rasterizer strips via `deleteEmptyColumnFor16`.
