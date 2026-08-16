package handler

import (
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const upstreamBaseURL = "https://appdownload.modelcolorresearch.club/"

var (
	upstreamURL = mustParseURL(upstreamBaseURL)
	httpClient  = &http.Client{
		Timeout: 0,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
	case http.MethodOptions:
		handleOptions(w)
		return
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := buildTargetURL(r)
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), nil)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}

	copyRequestHeaders(upstreamReq.Header, r.Header)

	resp, err := httpClient.Do(upstreamReq)
	if err != nil {
		status := http.StatusBadGateway
		if isTimeoutError(err) {
			status = http.StatusGatewayTimeout
		}
		log.Printf("proxy request failed: %v", err)
		http.Error(w, "upstream request failed", status)
		return
	}
	defer resp.Body.Close()

	setCORSHeaders(w.Header())
	copyResponseHeaders(w.Header(), resp.Header, targetURL.Path)
	w.WriteHeader(resp.StatusCode)

	if r.Method == http.MethodHead {
		return
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("response stream failed: %v", err)
	}
}

func handleOptions(w http.ResponseWriter) {
	setCORSHeaders(w.Header())
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, If-Modified-Since, If-None-Match, Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func buildTargetURL(r *http.Request) *url.URL {
	target := *upstreamURL
	target.Path = joinURLPath(upstreamURL.Path, requestPath(r))
	query := r.URL.Query()
	query.Del("path")
	target.RawQuery = query.Encode()
	return &target
}

func requestPath(r *http.Request) string {
	if rewrittenPath := strings.TrimSpace(r.URL.Query().Get("path")); rewrittenPath != "" {
		return "/" + strings.TrimPrefix(rewrittenPath, "/")
	}

	if r.URL.Path == "" || r.URL.Path == "/api/index" {
		return "/"
	}
	return r.URL.Path
}

func joinURLPath(basePath, reqPath string) string {
	cleanBase := strings.TrimSuffix(basePath, "/")
	cleanReq := path.Clean("/" + strings.TrimPrefix(reqPath, "/"))
	if cleanReq == "." {
		cleanReq = "/"
	}
	if cleanBase == "" {
		return cleanReq
	}
	if cleanReq == "/" {
		return cleanBase + "/"
	}
	return cleanBase + cleanReq
}

func copyRequestHeaders(dst, src http.Header) {
	allowed := []string{
		"Range",
		"If-Modified-Since",
		"If-None-Match",
		"If-Match",
		"If-Unmodified-Since",
		"If-Range",
		"User-Agent",
		"Accept",
		"Origin",
	}

	for _, key := range allowed {
		for _, value := range src.Values(key) {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst, src http.Header, targetPath string) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}

	if dst.Get("Content-Type") == "" {
		if contentType := mime.TypeByExtension(path.Ext(targetPath)); contentType != "" {
			dst.Set("Content-Type", contentType)
		}
	}
}

func setCORSHeaders(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Length, Content-Range, Content-Type, Content-Disposition, ETag, Last-Modified")
}

func isHopByHopHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te",
		"Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(interface{ Timeout() bool }); ok {
		return netErr.Timeout()
	}
	return false
}
