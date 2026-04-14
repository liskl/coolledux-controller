#!/usr/bin/env bash
# Extracts bitmap font and animation assets from the CoolLED 1248 APK into
# their respective //go:embed directories. These assets are not committed to
# this repo (third-party origin); this script must be run before `go build`.
set -euo pipefail

APK="references/coolled-1248.apk"
FONTS_OUT="internal/text/fonts"
COUNTDOWN_OUT="internal/controller/assets"

if [[ ! -f "$APK" ]]; then
    echo "error: $APK not found. See CLAUDE.local.md for how to obtain it." >&2
    exit 1
fi

mkdir -p "$FONTS_OUT" "$COUNTDOWN_OUT"

unzip -j -o "$APK" \
    'assets/flutter_assets/assets/coolledux/font_library/unicode_16_bold' \
    -d "$FONTS_OUT"
mv "$FONTS_OUT/unicode_16_bold" "$FONTS_OUT/unicode_16_bold.bin"

unzip -j -o "$APK" \
    'res/drawable-xxhdpi-v4/ic_countdown_bg_animation_1696.gif' \
    -d "$COUNTDOWN_OUT"
mv "$COUNTDOWN_OUT/ic_countdown_bg_animation_1696.gif" \
    "$COUNTDOWN_OUT/countdown_bg_1696.gif"

unzip -j -o "$APK" \
    'res/drawable-xxhdpi-v4/ic_stopwatch_bg_animation_1696.gif' \
    -d "$COUNTDOWN_OUT"
mv "$COUNTDOWN_OUT/ic_stopwatch_bg_animation_1696.gif" \
    "$COUNTDOWN_OUT/stopwatch_bg_1696.gif"

echo "fonts:     $(ls "$FONTS_OUT"/*.bin 2>/dev/null || echo none)"
echo "overlays:  $(ls "$COUNTDOWN_OUT"/*.gif 2>/dev/null || echo none)"
