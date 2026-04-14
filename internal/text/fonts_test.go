package text

import (
	"testing"
)

func TestFaceReturnsRegisteredFont(t *testing.T) {
	for _, name := range []string{"7x13", "7x14b", "8x16"} {
		f, err := Face(name)
		if err != nil {
			t.Fatalf("Face(%q) err = %v", name, err)
		}
		if f == nil {
			t.Fatalf("Face(%q) returned nil face", name)
		}
	}
}

func TestFaceEmptyNameReturnsDefault(t *testing.T) {
	got, err := Face("")
	if err != nil {
		t.Fatalf("Face(\"\") err = %v", err)
	}
	want, err := Face(DefaultFontName)
	if err != nil {
		t.Fatalf("Face(default) err = %v", err)
	}
	if got != want {
		t.Fatalf("Face(\"\") should equal Face(DefaultFontName)")
	}
}

func TestFaceUnknownReturnsError(t *testing.T) {
	if _, err := Face("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown font name")
	}
}

func TestBaselineOffset(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"7x13", 0},
		{"7x14b", 0},
		{"8x16", 1}, // Spleen nudges down by 1
		{"", 0},     // default
		{"no-such-font", 0},
	}
	for _, tt := range tests {
		if got := BaselineOffset(tt.name); got != tt.want {
			t.Errorf("BaselineOffset(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestFontInfoFor(t *testing.T) {
	info, ok := FontInfoFor("7x13")
	if !ok {
		t.Fatal("FontInfoFor(7x13) ok=false")
	}
	if info.Name != "7x13" {
		t.Errorf("info.Name = %q, want 7x13", info.Name)
	}
	if !info.Monospace {
		t.Errorf("7x13 should be monospace")
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Errorf("7x13 size = %dx%d; expected positive", info.Width, info.Height)
	}

	// Empty name resolves to default
	def, ok := FontInfoFor("")
	if !ok || def.Name != DefaultFontName {
		t.Errorf("FontInfoFor(\"\") = %+v, ok=%v", def, ok)
	}

	// Unknown name returns zero value + false
	zero, ok := FontInfoFor("bogus")
	if ok {
		t.Errorf("FontInfoFor(bogus) ok=true, want false")
	}
	if zero != (FontInfo{}) {
		t.Errorf("FontInfoFor(bogus) = %+v, want zero", zero)
	}
}

func TestAvailableFontsSortedAndComplete(t *testing.T) {
	got := AvailableFonts()
	if len(got) < 3 {
		t.Fatalf("AvailableFonts returned %d fonts, want >= 3", len(got))
	}
	wantNames := map[string]bool{"7x13": false, "7x14b": false, "8x16": false}
	for i := 1; i < len(got); i++ {
		if got[i-1].Name >= got[i].Name {
			t.Errorf("AvailableFonts not sorted: %q >= %q", got[i-1].Name, got[i].Name)
		}
	}
	for _, info := range got {
		if _, ok := wantNames[info.Name]; ok {
			wantNames[info.Name] = true
		}
	}
	for name, seen := range wantNames {
		if !seen {
			t.Errorf("AvailableFonts missing %q", name)
		}
	}
}

func TestRegisterAndRegisterWithOffset(t *testing.T) {
	// Register a synthetic entry, then confirm Face/BaselineOffset/FontInfoFor
	// return it. Uses the existing 7x13 face as a stand-in.
	face, err := Face(DefaultFontName)
	if err != nil {
		t.Fatalf("Face(default) err = %v", err)
	}
	info := FontInfo{Name: "test-synth", Description: "synthetic", Width: 7, Height: 13, Monospace: true}
	RegisterWithOffset(info, face, 3)
	defer delete(registry, "test-synth")

	if got, err := Face("test-synth"); err != nil || got != face {
		t.Fatalf("Face(test-synth) err=%v got=%v", err, got)
	}
	if got := BaselineOffset("test-synth"); got != 3 {
		t.Errorf("BaselineOffset(test-synth) = %d, want 3", got)
	}
	if gotInfo, ok := FontInfoFor("test-synth"); !ok || gotInfo != info {
		t.Errorf("FontInfoFor(test-synth) = %+v ok=%v, want %+v", gotInfo, ok, info)
	}

	// Plain Register: zero offset.
	Register(FontInfo{Name: "test-zero", Width: 1, Height: 1}, face)
	defer delete(registry, "test-zero")
	if got := BaselineOffset("test-zero"); got != 0 {
		t.Errorf("BaselineOffset(test-zero) = %d, want 0", got)
	}
}
