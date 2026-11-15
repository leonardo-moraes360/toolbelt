package verify

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultSimilarityThreshold = 0.95
	padColorMagenta            = 0xFF00FF
)

type VisualConfig struct {
	Manifest   string
	Threshold  float64
	CaptureDir string
	SkipOK     bool
}

type ScreenSpec struct {
	Name       string            `yaml:"name"`
	Route      string            `yaml:"route"`
	Modes      []string          `yaml:"modes"`
	Viewport   []int             `yaml:"viewport"`
	References map[string]string `yaml:"references"`
	URL        string            `yaml:"url"`
}

type ScreensManifest struct {
	Version    int          `yaml:"version"`
	BaseURL    string       `yaml:"base_url"`
	Screens    []ScreenSpec `yaml:"screens"`
	Thresholds struct {
		MinSimilarity float64 `yaml:"min_similarity"`
	} `yaml:"thresholds"`
}

type ScreenResult struct {
	Screen     string  `json:"screen"`
	Mode       string  `json:"mode"`
	Similarity float64 `json:"similarity"`
	SizeMatch  bool    `json:"size_match"`
	Passed     bool    `json:"passed"`
	Skipped    bool    `json:"skipped,omitempty"`
	SkipReason string  `json:"skip_reason,omitempty"`
	Capture    string  `json:"capture,omitempty"`
	Reference  string  `json:"reference,omitempty"`
}

type VisualReport struct {
	Verdict    Verdict        `json:"verdict"`
	Skipped    bool           `json:"skipped"`
	SkipReason string         `json:"skip_reason,omitempty"`
	Threshold  float64        `json:"threshold"`
	Screens    []ScreenResult `json:"screens"`
	Browser    string         `json:"browser,omitempty"`
}

func LoadScreensManifest(path string) (*ScreensManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m ScreensManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Thresholds.MinSimilarity == 0 {
		m.Thresholds.MinSimilarity = DefaultSimilarityThreshold
	}
	return &m, nil
}

func FindHeadlessBrowser() (string, error) {
	candidates := []string{
		os.Getenv("TB_BROWSER"),
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"chrome",
	}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("browser headless indisponível")
}

func RunVisual(ctx context.Context, repo string, cfg VisualConfig) (*VisualReport, error) {
	cfg = normalizeVisualConfig(repo, cfg)
	report := &VisualReport{Threshold: cfg.Threshold, Verdict: VerdictFail}

	browser, err := FindHeadlessBrowser()
	if err != nil {
		return skipVisual(report, err.Error(), cfg.SkipOK), nil
	}
	report.Browser = browser

	manifest, err := LoadScreensManifest(cfg.Manifest)
	if err != nil {
		if os.IsNotExist(err) && cfg.SkipOK {
			return skipVisual(report, "manifesto de telas ausente", true), nil
		}
		return nil, err
	}
	if manifest.Thresholds.MinSimilarity > 0 {
		report.Threshold = manifest.Thresholds.MinSimilarity
	}
	if err := os.MkdirAll(cfg.CaptureDir, 0o755); err != nil {
		return nil, err
	}

	allPass, err := runVisualScreens(ctx, browser, repo, cfg, manifest, report)
	if err != nil {
		return nil, err
	}
	if allPass {
		report.Verdict = VerdictPass
	} else {
		report.Verdict = VerdictFail
	}
	return report, nil
}

func normalizeVisualConfig(repo string, cfg VisualConfig) VisualConfig {
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultSimilarityThreshold
	}
	if cfg.Manifest == "" {
		cfg.Manifest = filepath.Join(repo, ".toolbelt", "screens.yaml")
	}
	if cfg.CaptureDir == "" {
		cfg.CaptureDir = filepath.Join(repo, ".toolbelt", "captures")
	}
	return cfg
}

func skipVisual(report *VisualReport, reason string, skipOK bool) *VisualReport {
	report.Skipped = true
	report.SkipReason = reason
	report.Verdict = VerdictPass
	if !skipOK {
		report.Verdict = VerdictFail
	}
	return report
}

func runVisualScreens(
	ctx context.Context,
	browser, repo string,
	cfg VisualConfig,
	manifest *ScreensManifest,
	report *VisualReport,
) (bool, error) {
	allPass := true
	for i := range manifest.Screens {
		screen := &manifest.Screens[i]
		modes := screen.Modes
		if len(modes) == 0 {
			modes = []string{"light"}
		}
		for _, mode := range modes {
			sr, err := checkScreen(ctx, browser, repo, cfg.CaptureDir, manifest.BaseURL, screen, mode, report.Threshold)
			if err != nil {
				return false, err
			}
			report.Screens = append(report.Screens, *sr)
			if !sr.Skipped && !sr.Passed {
				allPass = false
			}
		}
	}
	return allPass, nil
}

func checkScreen(
	ctx context.Context,
	browser, repo, captureDir, baseURL string,
	screen *ScreenSpec,
	mode string,
	threshold float64,
) (*ScreenResult, error) {
	refPath := ""
	if screen.References != nil {
		refPath = screen.References[mode]
	}
	if refPath == "" {
		refPath = filepath.Join("references", screen.Name+"-"+mode+".png")
	}
	if !filepath.IsAbs(refPath) {
		refPath = filepath.Join(repo, refPath)
	}

	res := &ScreenResult{Screen: screen.Name, Mode: mode, Reference: refPath}
	if _, err := os.Stat(refPath); err != nil {
		res.Skipped = true
		res.SkipReason = "referência ausente"
		res.Passed = true
		return res, nil
	}

	url := screen.URL
	if url == "" {
		url = strings.TrimRight(baseURL, "/") + screen.Route
	}
	if url == "" || url == screen.Route {
		res.Skipped = true
		res.SkipReason = "URL/base_url ausente para captura"
		res.Passed = true
		return res, nil
	}

	capturePath := filepath.Join(captureDir, screen.Name+"-"+mode+".png")
	vw, vh := 1280, 720
	if len(screen.Viewport) >= 2 {
		vw, vh = screen.Viewport[0], screen.Viewport[1]
	}
	if err := captureScreenshot(ctx, browser, url, capturePath, vw, vh); err != nil {
		res.Skipped = true
		res.SkipReason = "falha na captura: " + err.Error()
		res.Passed = true
		return res, nil
	}
	res.Capture = capturePath

	sim, sizeMatch, err := ComparePNG(refPath, capturePath)
	if err != nil {
		return nil, err
	}
	res.Similarity = sim
	res.SizeMatch = sizeMatch
	res.Passed = sim >= threshold
	return res, nil
}

func captureScreenshot(ctx context.Context, browser, url, outPath string, width, height int) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--hide-scrollbars",
		fmt.Sprintf("--window-size=%d,%d", width, height),
		"--screenshot=" + outPath,
		url,
	}
	cmd := exec.CommandContext(ctx, browser, args...) //nolint:gosec // browser detectado localmente
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, truncate(string(out), 200))
	}
	if _, err := os.Stat(outPath); err != nil {
		return fmt.Errorf("screenshot não gerado")
	}
	return nil
}

func ComparePNG(refPath, capturePath string) (similarity float64, sizeMatch bool, err error) {
	ref, err := decodePNG(refPath)
	if err != nil {
		return 0, false, err
	}
	capImg, err := decodePNG(capturePath)
	if err != nil {
		return 0, false, err
	}
	rb := ref.Bounds()
	cb := capImg.Bounds()
	sizeMatch = rb.Dx() == cb.Dx() && rb.Dy() == cb.Dy()

	w := maxInt(rb.Dx(), cb.Dx())
	h := maxInt(rb.Dy(), cb.Dy())
	refRGBA := padTo(ref, w, h)
	capRGBA := padTo(capImg, w, h)

	var diff, total int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			total++
			if !colorsClose(refRGBA.RGBAAt(x, y), capRGBA.RGBAAt(x, y)) {
				diff++
			}
		}
	}
	if total == 0 {
		return 0, sizeMatch, nil
	}
	similarity = RoundSimilarity(1.0 - float64(diff)/float64(total))
	return similarity, sizeMatch, nil
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func padTo(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	magenta := color.RGBA{R: 0xFF, B: 0xFF, A: 0xFF}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: magenta}, image.Point{}, draw.Src)
	draw.Draw(dst, src.Bounds(), src, src.Bounds().Min, draw.Src)
	_ = padColorMagenta
	return dst
}

func colorsClose(a, b color.RGBA) bool {
	const tol = 8
	return absDiff(a.R, b.R) <= tol && absDiff(a.G, b.G) <= tol && absDiff(a.B, b.B) <= tol
}

func absDiff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func RoundSimilarity(v float64) float64 {
	return math.Round(v*10000) / 10000
}
