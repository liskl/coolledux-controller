# Controller Assets

Binary assets embedded via `//go:embed` at compile time. Files here are
extracted from the CoolLED 1248 Android APK and **not committed** to this
repo (see `.gitignore`). They must be present before `go build`.

Run `scripts/extract-assets.sh` to populate this directory. It requires
`references/coolled-1248.apk` (see `CLAUDE.local.md` for how to obtain it).

## Files

- `countdown_bg_1696.gif` — 96x16, 18-frame purple frame + hourglass animation.
  Source: `res/drawable-xxhdpi-v4/ic_countdown_bg_animation_1696.gif` in the
  APK. Used as the animation layer of the composite countdown program.
- `stopwatch_bg_1696.gif` — 96x16 stopwatch background animation.
  Source: `res/drawable-xxhdpi-v4/ic_stopwatch_bg_animation_1696.gif` in the
  APK. Used as the animation layer of the composite stopwatch program.
