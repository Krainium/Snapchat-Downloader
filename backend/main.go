// Snapchat spotlight and story downloader.
//
// A share link redirects to the public spotlight page, which embeds the whole
// post as JSON in a __NEXT_DATA__ script tag. The media urls live in there, so
// no API key and no login are involved. Downloads are streamed back through
// this server so the browser gets a real filename.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

var nextData = regexp.MustCompile(`(?s)__NEXT_DATA__"[^>]*>(.*?)</script>`)

// wire types
type Media struct {
	Type     string `json:"type"` // video | photo
	URL      string `json:"url"`
	Thumb    string `json:"thumb"`
	Ext      string `json:"ext"`
	Filename string `json:"filename"`
}

type Post struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Author  string  `json:"author"`
	Handle  string  `json:"handle"`
	Avatar  string  `json:"avatar"`
	PageURL string  `json:"pageUrl"`
	Media   []Media `json:"media"`
}

// http
func newClient() *http.Client {
	tr := &http.Transport{
		MaxIdleConns:        64,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}
	if p := os.Getenv("SDL_PROXY"); p != "" {
		if u, err := url.Parse(p); err == nil {
			tr.Proxy = http.ProxyURL(u)
			log.Printf("upstream proxy: %s", u.Redacted())
		}
	}
	return &http.Client{Transport: tr, Timeout: 60 * time.Second}
}

var client = newClient()

// browserHeaders makes a request look like a real navigation. Snapchat is
// lenient, but the same shape is used everywhere for consistency.
func browserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

// extraction
func validLink(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return errors.New("that does not look like a Snapchat link")
	}
	h := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(h, "snapchat.com") {
		return errors.New("only snapchat.com links are supported")
	}
	return nil
}

// walk pulls the first string value under any of the wanted keys. The payload
// shape shifts between spotlight, story and profile pages, so the search is by
// key name rather than a fixed path.
func walk(node any, wanted map[string]bool, out map[string]string) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if s, ok := child.(string); ok && wanted[k] && out[k] == "" && strings.HasPrefix(s, "http") {
				out[k] = s
				continue
			}
			walk(child, wanted, out)
		}
	case []any:
		for _, child := range v {
			walk(child, wanted, out)
		}
	}
}

// asString returns node as a string when it is one, else "".
func asString(v any) string {
	s, _ := v.(string)
	return s
}

// digMap walks nested maps by key, returning nil the moment a hop is missing or
// is not a map. Reading a key from the nil result is still safe in Go, so calls
// can be chained without guarding every level.
func digMap(node any, keys ...string) map[string]any {
	for _, k := range keys {
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[k]
	}
	m, _ := node.(map[string]any)
	return m
}

// targetMeta returns the metadata object for the snap that was actually asked
// for.
//
// A spotlight page embeds a whole recommendation feed, dozens of unrelated
// snaps, not just the requested one. Taking "the first media url in the tree"
// is wrong twice over: it is a random snap, and because Go randomises map
// iteration it is a different random snap on every request. The requested snap
// is located by matching query.snapID against each story's storyId, which is
// exact and stable.
func targetMeta(props map[string]any, wantSnap string) map[string]any {
	stories, _ := digMap(props, "spotlightFeed")["spotlightStories"].([]any)

	// 1. Exact match on the requested snap id.
	if wantSnap != "" {
		for _, it := range stories {
			st, _ := it.(map[string]any)
			if asString(digMap(st, "story", "storyId")["value"]) == wantSnap {
				if md := digMap(st, "metadata"); md != nil {
					return md
				}
			}
		}
	}

	// 2. The top level videoMetadata, which Snapchat sets to the requested snap.
	if vm := digMap(props, "videoMetadata"); asString(vm["contentUrl"]) != "" {
		return map[string]any{"videoMetadata": vm}
	}

	// 3. The first story in the feed, only as a last resort.
	if len(stories) > 0 {
		if md := digMap(stories[0], "metadata"); md != nil {
			return md
		}
	}
	return nil
}

func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
	if len(s) > 60 {
		s = s[:60]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = "snapchat"
	}
	return s
}

func extract(rawURL string) (*Post, error) {
	if err := validLink(rawURL); err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", strings.TrimSpace(rawURL), nil)
	browserHeaders(req)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Snapchat: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Snapchat returned %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 12<<20))
	if err != nil {
		return nil, err
	}

	m := nextData.FindSubmatch(body)
	if m == nil {
		return nil, errors.New("could not read the post, it may be private or removed")
	}
	var doc map[string]any
	if err := json.Unmarshal(m[1], &doc); err != nil {
		return nil, errors.New("unreadable response from Snapchat")
	}

	// The final url and its query carry the id of the snap that was requested,
	// which is what pins extraction to the right video.
	query := digMap(doc, "query")
	wantSnap := asString(query["snapID"])
	props := digMap(doc, "props", "pageProps")

	md := targetMeta(props, wantSnap)
	vm := digMap(md, "videoMetadata")
	video := asString(vm["contentUrl"])

	var thumb, handle, author string
	if video != "" {
		thumb = asString(vm["thumbnailUrl"])
		if cr := digMap(vm, "creator", "personCreator"); cr != nil {
			handle = asString(cr["username"])
			author = asString(cr["name"])
		}
	} else {
		// Non spotlight page (a plain story or profile). Fall back to a generic
		// search; these pages carry a single snap, so order does not matter.
		urls := map[string]string{}
		walk(doc, map[string]bool{"mediaUrl": true, "contentUrl": true, "thumbnailUrl": true}, urls)
		if video = urls["mediaUrl"]; video == "" {
			video = urls["contentUrl"]
		}
		thumb = urls["thumbnailUrl"]
	}
	if video == "" {
		return nil, errors.New("this post has no downloadable video")
	}

	title := asString(vm["name"])
	if title == "" || title == "Spotlight Snap" {
		title = asString(md["llmTitle"])
	}

	if handle == "" {
		handle = strings.TrimPrefix(asString(query["username"]), "@")
	}
	if handle == "" {
		if p := strings.Split(res.Request.URL.Path, "/"); len(p) > 1 {
			handle = strings.TrimPrefix(p[1], "@")
		}
	}
	if author == "" {
		author = handle
	}

	id := wantSnap
	if id == "" {
		id = strings.Trim(res.Request.URL.Path, "/")
	}

	base := safeName(handle)
	return &Post{
		ID:      id,
		Title:   title,
		Author:  author,
		Handle:  handle,
		PageURL: res.Request.URL.String(),
		Media: []Media{{
			Type:     "video",
			URL:      video,
			Thumb:    thumb,
			Ext:      "mp4",
			Filename: base + "_snapchat.mp4",
		}},
	}, nil
}

// handlers
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"ok":      true,
		"service": "snapchat",
		"proxied": os.Getenv("SDL_PROXY") != "",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func handleExtract(w http.ResponseWriter, r *http.Request) {
	post, err := extract(r.URL.Query().Get("url"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, post)
}

// Only Snapchat's own media hosts may be fetched, so this cannot be used as an
// open proxy for arbitrary urls.
func allowedMediaHost(h string) bool {
	h = strings.ToLower(h)
	for _, s := range []string{".sc-cdn.net", "sc-cdn.net", ".snapchat.com", "snapchat.com"} {
		if strings.HasSuffix(h, s) {
			return true
		}
	}
	return false
}

func proxyMedia(w http.ResponseWriter, r *http.Request, attachment bool) {
	target := r.URL.Query().Get("url")
	name := r.URL.Query().Get("filename")
	if target == "" {
		writeJSON(w, 400, map[string]string{"error": "url is required"})
		return
	}
	u, err := url.Parse(target)
	if err != nil || !allowedMediaHost(u.Hostname()) {
		writeJSON(w, 403, map[string]string{"error": "only Snapchat media hosts are allowed"})
		return
	}

	req, _ := http.NewRequest("GET", target, nil)
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Referer", "https://www.snapchat.com/")
	req.Header.Set("Accept", "*/*")
	if rng := r.Header.Get("Range"); rng != "" && !attachment {
		req.Header.Set("Range", rng)
	}

	res, err := client.Do(req)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": "could not fetch media"})
		return
	}
	defer res.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := res.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if attachment {
		if name == "" {
			name = "snapchat.mp4"
		}
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(safeName(name))))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("Cache-Control", "private, max-age=600")
		w.WriteHeader(res.StatusCode)
	}
	_, _ = io.Copy(w, res.Body)
}

func handleDownload(w http.ResponseWriter, r *http.Request) { proxyMedia(w, r, true) }
func handleMedia(w http.ResponseWriter, r *http.Request)    { proxyMedia(w, r, false) }

func staticHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "4446"
	}
	dist := os.Getenv("SDL_DIST")
	if dist == "" {
		dist = "./public"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/extract", handleExtract)
	mux.HandleFunc("/api/download", handleDownload)
	mux.HandleFunc("/api/media", handleMedia)
	mux.Handle("/", staticHandler(dist))

	srv := &http.Server{Addr: ":" + port, Handler: withLogging(mux), ReadHeaderTimeout: 15 * time.Second}
	log.Printf("snapchat-downloader listening on :%s (serving %s)", port, dist)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
