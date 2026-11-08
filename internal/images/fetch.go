// Package images fetches, caches, scales and kitty-encodes post images.
// It has no Bubble Tea dependency.
package images

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

var (
	ErrScheme   = errors.New("only http and https images are supported")
	ErrNotImage = errors.New("response is not an image")
	ErrTooLarge = errors.New("image is too large")
)

// Fetcher loads images over HTTP through an on-disk cache.
type Fetcher struct {
	dir      string
	client   *http.Client
	maxBytes int64
}

func NewFetcher(cacheDir string, maxBytes int64, timeout time.Duration) *Fetcher {
	return &Fetcher{dir: cacheDir, client: &http.Client{Timeout: timeout}, maxBytes: maxBytes}
}

// DefaultCacheDir is $XDG_CACHE_HOME/izrss/images, created if needed.
func DefaultCacheDir() (string, error) {
	marker, err := xdg.CacheFile("izrss/images/.keep")
	if err != nil {
		return "", fmt.Errorf("getting image cache dir: %w", err)
	}
	return filepath.Dir(marker), nil
}

// Load returns the image at rawURL, from cache when possible. Cache errors
// are ignored: the image is fetched over HTTP instead.
func (f *Fetcher) Load(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: %q", ErrScheme, rawURL)
	}

	path := filepath.Join(f.dir, cacheKey(rawURL))
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	data, err := f.fetch(rawURL)
	if err != nil {
		return nil, err
	}
	writeCache(path, data)
	return data, nil
}

func (f *Fetcher) fetch(rawURL string) ([]byte, error) {
	resp, err := f.client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetching image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching image: %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("%w: %q", ErrNotImage, ct)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}
	if int64(len(data)) > f.maxBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}

func cacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

// writeCache stores data atomically; failures only cost a future re-fetch.
func writeCache(path string, data []byte) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		_ = os.Remove(tmp.Name())
	}
}
