package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha256Matches(path, expected string) bool {
	if expected == "" || expected == "<hex>" {
		return true
	}
	got, err := sha256File(path)
	if err != nil {
		return false
	}
	return got == expected
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n2 := n / unit; n2 >= unit; n2 /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func formatETA(remaining int64, rate int64) string {
	if rate <= 0 || remaining <= 0 {
		return "?"
	}
	sec := remaining / rate
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

type progressWriter struct {
	name      string
	total     int64
	written   int64
	lastTick  time.Time
	lastBytes int64
	out       *os.File
	isTTY     bool
}

func newProgressWriter(name string, total int64, out *os.File) *progressWriter {
	return &progressWriter{
		name:     name,
		total:    total,
		out:      out,
		isTTY:    isTTY(out),
		lastTick: time.Now(),
	}
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	_, err = unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

func (p *progressWriter) redraw(line string) {
	// \r + line + clear-to-EOL so shorter finals never leave garbage
	_, _ = fmt.Fprintf(p.out, "\r%s\033[K", line)
}

func (p *progressWriter) Write(buf []byte) (int, error) {
	n := len(buf)
	p.written += int64(n)

	if !p.isTTY {
		return n, nil
	}

	now := time.Now()
	if now.Sub(p.lastTick) < 200*time.Millisecond && (p.total <= 0 || p.written < p.total) {
		return n, nil
	}
	elapsed := now.Sub(p.lastTick)
	rate := int64(0)
	if elapsed > 0 {
		rate = (p.written - p.lastBytes) * int64(time.Second) / int64(elapsed)
	}
	p.lastTick = now
	p.lastBytes = p.written

	var line string
	if p.total > 0 {
		pct := p.written * 100 / p.total
		if pct > 100 {
			pct = 100
		}
		remaining := p.total - p.written
		line = fmt.Sprintf("%s:  %3d%%  %s/%s  %s/s  eta %s",
			p.name, pct, humanSize(p.written), humanSize(p.total),
			humanSize(rate), formatETA(remaining, rate))
	} else {
		line = fmt.Sprintf("%s:  ?%%  %s  %s/s",
			p.name, humanSize(p.written), humanSize(rate))
	}
	p.redraw(line)
	return n, nil
}

func (p *progressWriter) Finish() {
	if !p.isTTY {
		return
	}
	size := p.written
	if p.total > 0 {
		size = p.total
	}
	line := fmt.Sprintf("%s:  100%%  %s/%s  done", p.name, humanSize(size), humanSize(size))
	p.redraw(line)
	_, _ = fmt.Fprintln(p.out)
}

func download(dst string, art Artifact) error {
	if st, err := os.Stat(dst); err == nil && sha256Matches(dst, art.SHA256) {
		fmt.Printf("✓ %s cached (%s)\n", art.Name, humanSize(st.Size()))
		return nil
	}
	_ = os.Remove(dst)

	fmt.Printf("Downloading %s (%s) from %s\n", art.Name, art.Version, art.URL)

	backoffs := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			fmt.Printf("retry %d/3 after %v…\n", attempt+1, backoffs[attempt-1])
			time.Sleep(backoffs[attempt-1])
		}
		lastErr = downloadOnce(dst+".tmp", art)
		if lastErr != nil {
			continue
		}
		fmt.Printf("verifying sha256…\n")
		if !sha256Matches(dst+".tmp", art.SHA256) {
			got, _ := sha256File(dst + ".tmp")
			_ = os.Remove(dst + ".tmp")
			fmt.Printf("sha256 mismatch (expected %s, got %s)\n", art.SHA256, got)
			lastErr = fmt.Errorf("sha256 mismatch")
			continue
		}
		if err := os.Rename(dst+".tmp", dst); err != nil {
			return err
		}
		st, _ := os.Stat(dst)
		fmt.Printf("✓ %s %s\n", art.Name, humanSize(st.Size()))
		return nil
	}
	return fmt.Errorf("failed to download %s (%s) after 3 attempts:\n  url:    %s\n  sha256: %s\n  last error: %v",
		art.Name, art.Version, art.URL, art.SHA256, lastErr)
}

func downloadOnce(dst string, art Artifact) error {
	req, err := http.NewRequest(http.MethodGet, art.URL, nil)
	if err != nil {
		return err
	}
	var offset int64
	if st, err := os.Stat(dst); err == nil {
		offset = st.Size()
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(dst, flags, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}

	total := resp.ContentLength
	if offset > 0 && total > 0 {
		total += offset
	}
	pw := newProgressWriter(art.Name, total, os.Stderr)
	_, err = io.Copy(io.MultiWriter(f, pw), resp.Body)
	if err != nil {
		return err
	}
	pw.Finish()
	return nil
}

func ensureSymlink(link, target string) error {
	_ = os.Remove(link)
	return os.Symlink(target, link)
}
