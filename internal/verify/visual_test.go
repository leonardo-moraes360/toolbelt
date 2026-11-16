package verify

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestComparePNGIdentical(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	writeSolidPNG(t, a, 10, 10, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	writeSolidPNG(t, b, 10, 10, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	sim, sizeMatch, err := ComparePNG(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !sizeMatch || sim < 0.999 {
		t.Fatalf("sim=%f sizeMatch=%v", sim, sizeMatch)
	}
}

func TestComparePNGDifferent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	writeSolidPNG(t, a, 8, 8, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	writeSolidPNG(t, b, 8, 8, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	sim, _, err := ComparePNG(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if sim > 0.1 {
		t.Fatalf("expected low similarity, got %f", sim)
	}
}

func TestComparePNGSizeDiff(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	writeSolidPNG(t, a, 10, 10, color.RGBA{R: 1, G: 1, B: 1, A: 255})
	writeSolidPNG(t, b, 10, 8, color.RGBA{R: 1, G: 1, B: 1, A: 255})
	sim, sizeMatch, err := ComparePNG(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if sizeMatch {
		t.Fatal("expected size mismatch")
	}
	if sim >= 1 {
		t.Fatalf("padding should reduce score: %f", sim)
	}
}

func TestFindHeadlessBrowserGraceful(t *testing.T) {
	t.Setenv("TB_BROWSER", "this-browser-does-not-exist-xyz")
	t.Setenv("PATH", t.TempDir())
	_, err := FindHeadlessBrowser()
	if err == nil {
		t.Fatal("expected missing browser error")
	}
}

func TestRunVisualSkipWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TB_BROWSER", "this-browser-does-not-exist-xyz")
	t.Setenv("PATH", dir)
	rep, err := RunVisual(context.Background(), dir, VisualConfig{SkipOK: true})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Skipped || rep.Verdict != VerdictSkipped {
		t.Fatalf("expected graceful SKIPPED, got %+v", rep)
	}
}

func TestRunVisualEmptyScreensSkipped(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(bin, "chromium")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TB_BROWSER", fake)
	t.Setenv("PATH", bin)

	toolbelt := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(toolbelt, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(toolbelt, "screens.yaml")
	content := []byte("version: 1\nbase_url: http://127.0.0.1:9\nscreens: []\n")
	if err := os.WriteFile(manifest, content, 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := RunVisual(context.Background(), dir, VisualConfig{SkipOK: true})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Skipped {
		t.Fatalf("expected top-level skipped=true for screens: [], got %+v", rep)
	}
	if rep.SkipReason == "" {
		t.Fatal("expected skip_reason for empty screens")
	}
	if rep.Verdict != VerdictSkipped {
		t.Fatalf("expected SKIPPED for empty screens, got %s", rep.Verdict)
	}
	if rep.Screens == nil {
		t.Fatal("expected screens=[] not null")
	}
	if len(rep.Screens) != 0 {
		t.Fatalf("expected 0 screen results, got %d", len(rep.Screens))
	}
}

func TestRunVisualAllScreensSkippedAggregates(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(bin, "chromium")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TB_BROWSER", fake)
	t.Setenv("PATH", bin)

	toolbelt := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(toolbelt, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(toolbelt, "screens.yaml")
	content := []byte("version: 1\nbase_url: http://127.0.0.1:9\nscreens:\n  - name: login\n    route: /login\n    modes: [light]\n  - name: home\n    route: /\n    modes: [dark]\n")
	if err := os.WriteFile(manifest, content, 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := RunVisual(context.Background(), dir, VisualConfig{SkipOK: true})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Skipped {
		t.Fatalf("expected top-level skipped=true when all screens skipped, got %+v", rep)
	}
	if rep.SkipReason == "" {
		t.Fatal("expected skip_reason when all screens skipped")
	}
	if rep.Verdict != VerdictSkipped {
		t.Fatalf("expected SKIPPED when all screens skipped, got %s", rep.Verdict)
	}
	if len(rep.Screens) != 2 {
		t.Fatalf("expected 2 screen results, got %d", len(rep.Screens))
	}
	for _, s := range rep.Screens {
		if !s.Skipped {
			t.Fatalf("expected screen skipped: %+v", s)
		}
	}
}

func TestAggregateVisualSkippedPartial(t *testing.T) {
	rep := &VisualReport{
		Screens: []ScreenResult{
			{Screen: "a", Skipped: true, Passed: true},
			{Screen: "b", Skipped: false, Passed: true, Similarity: 0.99},
		},
	}
	aggregateVisualSkipped(rep)
	if rep.Skipped {
		t.Fatal("partial skip must not set top-level skipped")
	}
}

func TestAggregateVisualSkippedEmpty(t *testing.T) {
	rep := &VisualReport{Screens: []ScreenResult{}}
	aggregateVisualSkipped(rep)
	if !rep.Skipped {
		t.Fatal("empty screens must set top-level skipped")
	}
	if rep.SkipReason == "" {
		t.Fatal("expected skip_reason for empty screens")
	}
}

func TestVisualVerdictSkippedNotPass(t *testing.T) {
	if got := visualVerdict(true, true, true); got != VerdictSkipped {
		t.Fatalf("skipped+skipOK => SKIPPED, got %s", got)
	}
	if got := visualVerdict(true, true, false); got != VerdictFail {
		t.Fatalf("skipped without skipOK => FAIL, got %s", got)
	}
	if got := visualVerdict(false, true, true); got != VerdictPass {
		t.Fatalf("not skipped + allPass => PASS, got %s", got)
	}
}

func TestRunVisualSkipWithoutManifestFailsWhenNotOK(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TB_BROWSER", "this-browser-does-not-exist-xyz")
	t.Setenv("PATH", dir)
	rep, err := RunVisual(context.Background(), dir, VisualConfig{SkipOK: false})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Skipped || rep.Verdict != VerdictFail {
		t.Fatalf("expected FAIL when skip-unavailable=false, got %+v", rep)
	}
}

func TestLoadScreensManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screens.yaml")
	content := []byte("version: 1\nbase_url: http://127.0.0.1:3000\nscreens:\n  - name: login\n    route: /login\n    modes: [light]\n    references:\n      light: references/login-light.png\nthresholds:\n  min_similarity: 0.95\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadScreensManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Screens) != 1 || m.Screens[0].Name != "login" {
		t.Fatalf("%+v", m)
	}
}

func writeSolidPNG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
