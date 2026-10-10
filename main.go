package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/cookiejar"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	colorReset   = "\033[0m"
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorPurple  = "\033[35m"
	colorRed     = "\033[31m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"

	progressBarGreen = "\033[38;2;120;220;120m"
	progressBarGray  = "\033[38;2;90;90;90m"
	pipSpeedCoral    = "\033[38;2;255;107;107m"
	pipTimeAmber     = "\033[38;2;255;200;120m"

	logoASCII = `
  [cyan]███████╗ █████╗     [green]██████╗  ██████╗ ██╗    ██╗███╗   ██╗██╗      ██████╗  █████╗ ██████╗ 
  [cyan]██╔════╝██╔══██╗    [green]██╔══██╗██╔═══██╗██║    ██║████╗  ██║██║     ██╔═══██╗██╔══██╗██╔══██╗
  [cyan]█████╗  ███████║    [green]██║  ██║██║   ██║██║ █╗ ██║██╔██╗ ██║██║     ██║   ██║███████║██║  ██║
  [cyan]██╔══╝  ██╔══██║    [green]██║  ██║██║   ██║██║███╗██║██║╚██╗██║██║     ██║   ██║██╔══██║██║  ██║
  [cyan]██║     ██║  ██║    [green]██████╔╝╚██████╔╝╚███╔███╔╝██║ ╚████║███████╗╚██████╔╝██║  ██║██████╔╝
  [cyan]╚═╝     ╚═╝  ╚═╝    [green]╚═════╝  ╚═════╝  ╚══╝╚══╝ ╚═╝  ╚═══╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚═════╝ 
`
)

var (
	totalFiles int
	totalSize  int64
	version    string
	outputDir  string
	downloaded = make(map[string]bool)

	sharedCookies []*http.Cookie
	client        tls_client.HttpClient

	useColor = true
)

func init() {
	fmt.Print(colorizeMap(logoASCII))
	fmt.Println()

	jar, _ := cookiejar.New(nil)

	client, _ = tls_client.NewHttpClient(
		tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithCookieJar(jar),
		tls_client.WithTransportOptions(&tls_client.TransportOptions{
			DisableCompression: true,
		}),
	)
}

func main() {
	versionFlag := flag.String("version", "", "Font Awesome version to download, e.g. v7.3.1 (default: latest)")
	flag.StringVar(versionFlag, "v", "", "alias for -version")
	outputFlag := flag.String("output", "", "output directory; writes css/ and webfonts/ directly (no download/releases/vX.X.X/)")
	flag.StringVar(outputFlag, "o", "", "alias for -output")
	noColor := flag.Bool("no-color", false, "disable colored output")
	flag.Parse()

	useColor = !*noColor && isTerminal(os.Stdout)
	outputDir = strings.TrimSpace(*outputFlag)

	requestedVersion := normalizeVersion(*versionFlag)

	cPrintf(colorCyan, "Fetching cookies from fontawesome.com...\n")

	resp, err := doGetRequest("https://fontawesome.com/")
	if err != nil {
		cPrintf(colorYellow, "error: fetch homepage: %v\n", err)
		return
	}
	defer resp.Body.Close()

	sharedCookies = resp.Cookies()

	if u, err := url.Parse("https://fontawesome.com/"); err == nil {
		if jar := client.GetCookieJar(); jar != nil {
			jarCookies := jar.Cookies(u)
			sharedCookies = append(sharedCookies, jarCookies...)
		}
	}

	if len(sharedCookies) > 0 {
		fmt.Printf("Got %d cookies.\n", len(sharedCookies))
	}

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		cPrintf(colorYellow, "error: read response: %v\n", err)
		return
	}
	htmlStr := string(htmlBytes)

	cssRegex := regexp.MustCompile(`href=["'](https://site-assets\.fontawesome\.com/releases/(v[0-9a-zA-Z.-]+)/css/[^"']+\.css)["']`)
	matches := cssRegex.FindAllStringSubmatch(htmlStr, -1)

	if len(matches) == 0 {
		fmt.Println("error: no CSS assets found (blocked or page changed)")
		fmt.Println("response preview:", htmlStr[:min(500, len(htmlStr))])
		return
	}

	siteVersion := matches[0][2]
	if requestedVersion != "" {
		version = requestedVersion
		if version != siteVersion {
			fmt.Printf("Site: %s → download: %s\n",
				siteVersion, colorize(colorBold+colorGreen, version))
		} else {
			fmt.Printf("Version: %s\n", colorize(colorBold+colorGreen, version))
		}
	} else {
		version = siteVersion
		fmt.Printf("Version: %s (latest)\n", colorize(colorBold+colorGreen, version))
	}
	if outputDir != "" {
		fmt.Printf("Output:  %s/css, %s/webfonts\n", outputDir, outputDir)
	}

	cssURLs := make([]string, 0, len(matches)+1)
	hasAllCSS := false
	for _, match := range matches {
		cssURLStr := rewriteVersion(match[1], siteVersion, version)
		cssURLs = append(cssURLs, cssURLStr)
		if strings.HasSuffix(cssURLStr, "/css/all.css") {
			hasAllCSS = true
		}
	}

	if !hasAllCSS {
		allCSSURL := fmt.Sprintf("https://site-assets.fontawesome.com/releases/%s/css/all.css", version)
		cssURLs = append(cssURLs, allCSSURL)
		fmt.Printf("Found %d CSS files (+ all.css).\n\n", len(matches))
	} else {
		fmt.Printf("Found %d CSS files.\n\n", len(matches))
	}

	for _, cssURLStr := range cssURLs {
		err := processCSS(cssURLStr)
		if err != nil {
			cPrintf(colorYellow, "error: CSS %s: %v\n", cssURLStr, err)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	cPrintf(colorGreen+colorBold, "Done.\n")
	fmt.Printf("Version: %s\n", colorize(colorBold, version))
	if outputDir != "" {
		fmt.Printf("Output:  %s\n", colorize(colorBold, outputDir))
	}
	fmt.Printf("Files:   %d\n", totalFiles)
	fmt.Printf("Size:    %s\n", formatBytes(totalSize))
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func rewriteVersion(rawURL, from, to string) string {
	if from == to {
		return rawURL
	}
	return strings.Replace(rawURL, "/releases/"+from+"/", "/releases/"+to+"/", 1)
}

func doGetRequest(reqURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	if !strings.Contains(reqURL, "fontawesome.com/") || strings.Contains(reqURL, "site-assets") {
		req.Header.Set("Referer", "https://fontawesome.com/")
		for _, cookie := range sharedCookies {
			req.AddCookie(cookie)
		}
	}

	return client.Do(req)
}

func processCSS(cssURLStr string) error {
	if downloaded[cssURLStr] {
		return nil
	}

	cssPath, err := downloadFile(cssURLStr)
	if err != nil {
		return err
	}
	downloaded[cssURLStr] = true

	cssBytes, err := os.ReadFile(cssPath)
	if err != nil {
		return err
	}
	cssContent := string(cssBytes)

	urlRegex := regexp.MustCompile(`url\(['"]?([^'"()]+)['"]?\)`)
	fontMatches := urlRegex.FindAllStringSubmatch(cssContent, -1)

	cssURL, _ := url.Parse(cssURLStr)

	for _, fm := range fontMatches {
		rawFontURL := fm[1]
		cleanFontPath := strings.Split(rawFontURL, "?")[0]
		cleanFontPath = strings.Split(cleanFontPath, "#")[0]

		if cleanFontPath == "" || strings.HasPrefix(cleanFontPath, "data:") {
			continue
		}

		fontRelURL, err := url.Parse(cleanFontPath)
		if err != nil {
			continue
		}
		fontAbsURL := cssURL.ResolveReference(fontRelURL)
		fontURLStr := fontAbsURL.String()

		if !downloaded[fontURLStr] {
			_, err = downloadFile(fontURLStr)
			if err != nil {
				cPrintf(colorYellow, "error: font %s: %v\n", fontURLStr, err)
			} else {
				downloaded[fontURLStr] = true
			}
		}
	}
	return nil
}

func localPathForURL(fileURL string) (string, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}

	rel := strings.TrimPrefix(u.Path, "/")

	if outputDir != "" {
		prefix := "releases/" + version + "/"
		switch {
		case strings.HasPrefix(rel, prefix):
			rel = strings.TrimPrefix(rel, prefix)
		default:
			for _, marker := range []string{"css/", "webfonts/"} {
				if i := strings.Index(rel, marker); i >= 0 {
					rel = rel[i:]
					break
				}
			}
		}
		return filepath.Join(outputDir, filepath.FromSlash(rel)), nil
	}

	return filepath.Join("download", filepath.FromSlash(rel)), nil
}

func downloadFile(fileURL string) (string, error) {
	localPath, err := localPathForURL(fileURL)
	if err != nil {
		return "", err
	}
	displayName := filepath.Base(localPath)

	if info, err := os.Stat(localPath); err == nil {
		totalFiles++
		totalSize += info.Size()
		fmt.Printf("%s %s %s\n",
			colorize(colorYellow, "Skipping"),
			displayName,
			colorize(colorDim, fmt.Sprintf("(%s, cached)", formatBytes(info.Size()))),
		)
		return localPath, nil
	}

	err = os.MkdirAll(filepath.Dir(localPath), os.ModePerm)
	if err != nil {
		return "", err
	}

	resp, err := doGetRequest(fileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	total := responseContentLength(resp)
	if total > 0 {
		fmt.Fprintf(os.Stderr, "Downloading %s (%s)\n", displayName, formatBytes(total))
	} else {
		fmt.Fprintf(os.Stderr, "Downloading %s\n", displayName)
	}
	bar := newProgressBar(total)

	written, err := io.Copy(io.MultiWriter(out, bar), resp.Body)
	bar.Finish(written)
	if err != nil {
		return "", err
	}

	totalFiles++
	totalSize += written

	return localPath, nil
}

func responseContentLength(resp *http.Response) int64 {
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return -1
}

type progressBar struct {
	total    int64
	current  int64
	width    int
	start    time.Time
	lastDraw time.Time
	done     bool
}

func newProgressBar(total int64) *progressBar {
	b := &progressBar{
		total: total,
		width: 40,
		start: time.Now(),
	}
	b.draw()
	return b
}

func (b *progressBar) Write(p []byte) (int, error) {
	n := len(p)
	b.current += int64(n)
	now := time.Now()
	if now.Sub(b.lastDraw) >= 50*time.Millisecond || (b.total > 0 && b.current >= b.total) {
		b.draw()
		b.lastDraw = now
	}
	return n, nil
}

func (b *progressBar) Finish(final int64) {
	if b.done {
		return
	}
	b.done = true
	b.current = final
	if b.total <= 0 {
		b.total = final
	}
	b.draw()
	fmt.Fprint(os.Stderr, "\n")
}

func (b *progressBar) draw() {
	elapsed := time.Since(b.start).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	speed := float64(b.current) / elapsed

	filled := 0
	if b.total > 0 {
		filled = int(float64(b.width) * float64(b.current) / float64(b.total))
		if filled > b.width {
			filled = b.width
		}
		if b.current > 0 && filled == 0 {
			filled = 1
		}
	}

	bar := renderProgressBar(filled, b.width)

	var sizePlain string
	if b.total > 0 {
		sizePlain = formatBytesPair(b.current, b.total)
	} else {
		sizePlain = formatBytes(b.current)
	}
	speedPlain := formatBytes(int64(speed)) + "/s"

	var timePlain string
	if b.done || (b.total > 0 && b.current >= b.total) {
		timePlain = formatPipDuration(elapsed)
	} else if b.total > 0 && speed > 0 {
		remain := float64(b.total-b.current) / speed
		timePlain = formatPipDuration(remain)
	} else {
		timePlain = "--:--"
	}

	sizePart := fmt.Sprintf("%-14s", sizePlain)
	speedPart := fmt.Sprintf("%-11s", speedPlain)
	timePart := fmt.Sprintf("%8s", timePlain)

	if useColor {
		sizePart = progressBarGreen + sizePart + colorReset
		speedPart = pipSpeedCoral + speedPart + colorReset
		timePart = pipTimeAmber + timePart + colorReset
	}

	fmt.Fprintf(os.Stderr, "\r\033[K   %s %s %s %s", bar, sizePart, speedPart, timePart)
}

func renderProgressBar(filled, width int) string {
	block := "━"
	if !useColor {
		return strings.Repeat(block, filled) + strings.Repeat("-", width-filled)
	}
	return progressBarGreen + strings.Repeat(block, filled) +
		progressBarGray + strings.Repeat(block, width-filled) +
		colorReset
}

func formatBytesPair(current, total int64) string {
	const unit = 1000.0
	units := []string{"B", "kB", "MB", "GB", "TB"}

	v := float64(total)
	exp := 0
	for v >= unit && exp < len(units)-1 {
		v /= unit
		exp++
	}
	div := 1.0
	for i := 0; i < exp; i++ {
		div *= unit
	}

	if exp == 0 {
		return fmt.Sprintf("%d/%d %s", current, total, units[0])
	}
	return fmt.Sprintf("%.1f/%.1f %s", float64(current)/div, float64(total)/div, units[exp])
}

func formatPipDuration(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

func formatBytes(bytes int64) string {
	const unit = 1000.0
	n := float64(bytes)
	if n < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	exp := 0
	for n >= unit && exp < 5 {
		n /= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", n, "kMGTPE"[exp-1])
}

func colorize(color, s string) string {
	if !useColor {
		return s
	}
	return color + s + colorReset
}

func colorizeMap(s string) string {
	colorMap := map[string]string{
		"[cyan]":    colorCyan,
		"[purple]":  colorPurple,
		"[green]":   colorGreen,
		"[yellow]":  colorYellow,
		"[red]":     colorRed,
		"[blue]":    colorBlue,
		"[magenta]": colorMagenta,
	}
	for color, value := range colorMap {
		s = strings.Replace(s, color, value, -1)
	}
	return s + colorReset
}

func cPrintf(color, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if useColor {
		fmt.Print(color + msg + colorReset)
	} else {
		fmt.Print(msg)
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
