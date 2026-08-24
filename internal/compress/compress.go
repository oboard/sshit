// Package compress provides an HTTP middleware that negotiates the best
// response encoding (zstd, br, gzip, deflate) from the request's
// Accept-Encoding header and compresses responses on the fly.
//
// Ported from the gin-based middleware in ticking-server-go; this version
// targets the standard library net/http only.
package compress

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// minCompressBytes is the threshold below which responses are passed through
// without compression. Small payloads don't benefit enough from compression
// to justify the CPU and allocation cost of initializing a writer.
const minCompressBytes = 2048

type compressionWriter interface {
	io.WriteCloser
	Flush() error
}

type encodedResponseWriter struct {
	http.ResponseWriter
	encoding string
	encoder  compressionWriter
	// buf holds the first minCompressBytes of response data while we decide
	// whether to compress. Once the threshold is exceeded, buf is flushed
	// into the real encoder and set to nil.
	buf             []byte
	passthrough     bool
	statusWritten   bool
	statusCode      int
	headerFinalized bool
}

// Middleware negotiates the best response encoding from the request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding := negotiateEncoding(r.Header.Get("Accept-Encoding"))
		if encoding == "" || isWebSocketRequest(r) || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		writer := &encodedResponseWriter{
			ResponseWriter: w,
			encoding:       encoding,
		}
		defer writer.Close()

		next.ServeHTTP(writer, r)
	})
}

// MiddlewareFunc is a convenience wrapper for http.HandlerFunc.
func MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return Middleware(next).ServeHTTP
}

func (w *encodedResponseWriter) WriteHeader(code int) {
	w.statusWritten = true
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *encodedResponseWriter) WriteHeaderNow() {
	if !w.statusWritten {
		w.statusWritten = true
		w.statusCode = http.StatusOK
	}
	if w.encoder == nil && !w.headerFinalized {
		w.disableCompression()
	}
}

func (w *encodedResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *encodedResponseWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	// While we're still below the threshold and haven't decided yet,
	// accumulate data into the buffer.
	if !w.passthrough && !w.headerFinalized {
		w.buf = append(w.buf, data...)
		if len(w.buf) < minCompressBytes {
			// Pretend we wrote the full slice; we'll flush the buffer later.
			return len(data), nil
		}
		// Threshold crossed — commit the buffered bytes through the encoder.
		if err := w.initEncoder(w.buf); err != nil {
			w.disableCompression()
		}
		if w.passthrough || w.encoder == nil {
			n, err := w.ResponseWriter.Write(w.buf)
			w.buf = nil
			if err != nil {
				return 0, err
			}
			// Report original data length so callers stay consistent.
			_ = n
			return len(data), nil
		}
		w.Header().Del("Content-Length")
		_, err := w.encoder.Write(w.buf)
		w.buf = nil
		if err != nil {
			return 0, err
		}
		return len(data), nil
	}

	if w.passthrough || w.encoder == nil {
		return w.ResponseWriter.Write(data)
	}

	w.Header().Del("Content-Length")
	return w.encoder.Write(data)
}

func (w *encodedResponseWriter) Flush() {
	// If we still have buffered data below the threshold, flush it uncompressed.
	if len(w.buf) > 0 {
		w.disableCompression()
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}

	if w.encoder != nil && !w.passthrough {
		_ = w.encoder.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *encodedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}

func (w *encodedResponseWriter) Close() error {
	// Flush any buffered data that never reached the threshold — send plain.
	if len(w.buf) > 0 {
		w.disableCompression()
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}
	if w.encoder == nil {
		return nil
	}
	err := w.encoder.Close()
	w.encoder = nil
	return err
}

// initEncoder runs the normal pre-flight checks and, if all pass, creates the
// real compression writer. It is called exactly once, when buffered data
// exceeds minCompressBytes.
func (w *encodedResponseWriter) initEncoder(data []byte) error {
	if w.passthrough || w.encoder != nil || w.headerFinalized {
		return nil
	}

	statusCode := w.currentStatus()
	if statusCode >= http.StatusBadRequest {
		w.disableCompression()
		return nil
	}

	header := w.Header()
	if header.Get("Content-Encoding") != "" {
		w.disableCompression()
		return nil
	}

	contentType := header.Get("Content-Type")
	if contentType == "" && len(data) > 0 {
		contentType = http.DetectContentType(data)
		header.Set("Content-Type", contentType)
	}

	if shouldSkipContentType(contentType) {
		w.disableCompression()
		return nil
	}

	encoder, err := newCompressionWriter(w.ResponseWriter, w.encoding)
	if err != nil {
		return err
	}

	header.Set("Content-Encoding", w.encoding)
	addVaryAcceptEncoding(header)
	header.Del("Content-Length")

	w.encoder = encoder
	w.headerFinalized = true
	return nil
}

func (w *encodedResponseWriter) currentStatus() int {
	if w.statusWritten {
		return w.statusCode
	}
	return http.StatusOK
}

func (w *encodedResponseWriter) disableCompression() {
	w.passthrough = true
	w.headerFinalized = true
}

func newCompressionWriter(dst io.Writer, encoding string) (compressionWriter, error) {
	switch encoding {
	case "zstd":
		// Use SpeedFastest (level 1) instead of the default SpeedDefault (level 3).
		// Trading a few percent compression ratio for significantly lower
		// encoder cost is worth it on the hot path.
		return zstd.NewWriter(dst, zstd.WithEncoderLevel(zstd.SpeedFastest))
	case "br":
		return brotli.NewWriter(dst), nil
	case "gzip":
		return gzip.NewWriter(dst), nil
	case "deflate":
		return flate.NewWriter(dst, flate.DefaultCompression)
	default:
		return nil, errors.New("unsupported content encoding")
	}
}

func shouldSkipContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		return false
	}

	if strings.HasPrefix(contentType, "text/event-stream") {
		return true
	}

	skippedPrefixes := []string{
		"image/",
		"audio/",
		"video/",
		"application/zip",
		"application/gzip",
		"application/x-gzip",
		"application/x-rar-compressed",
		"application/vnd.rar",
		"application/x-7z-compressed",
		"application/x-brotli",
		"application/zstd",
		"application/x-zstd",
		"font/",
	}

	for _, prefix := range skippedPrefixes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}

	return false
}

func isWebSocketRequest(r *http.Request) bool {
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}

func addVaryAcceptEncoding(header http.Header) {
	current := header.Values("Vary")
	for _, value := range current {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Accept-Encoding") {
				return
			}
		}
	}

	header.Add("Vary", "Accept-Encoding")
}

func negotiateEncoding(acceptEncoding string) string {
	if strings.TrimSpace(acceptEncoding) == "" {
		return ""
	}

	supported := []string{"zstd", "br", "gzip", "deflate"}
	specs := parseAcceptEncodings(acceptEncoding)
	bestEncoding := ""
	bestQ := -1.0

	for _, offer := range supported {
		if q, ok := qualityForEncoding(specs, offer); ok && q > bestQ {
			bestQ = q
			bestEncoding = offer
		}
	}

	if bestQ <= 0 {
		return ""
	}

	return bestEncoding
}

type acceptEncodingSpec struct {
	value string
	q     float64
}

func parseAcceptEncodings(header string) []acceptEncodingSpec {
	parts := strings.Split(header, ",")
	specs := make([]acceptEncodingSpec, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		value := part
		q := 1.0

		if idx := strings.Index(part, ";"); idx >= 0 {
			value = strings.TrimSpace(part[:idx])
			params := strings.Split(part[idx+1:], ";")
			for _, param := range params {
				param = strings.TrimSpace(param)
				if !strings.HasPrefix(strings.ToLower(param), "q=") {
					continue
				}

				parsedQ, err := strconv.ParseFloat(strings.TrimSpace(param[2:]), 64)
				if err == nil {
					q = parsedQ
				}
				break
			}
		}

		if value == "" {
			continue
		}

		specs = append(specs, acceptEncodingSpec{
			value: strings.ToLower(value),
			q:     q,
		})
	}

	return specs
}

func qualityForEncoding(specs []acceptEncodingSpec, encoding string) (float64, bool) {
	encoding = strings.ToLower(encoding)
	found := false
	bestQ := 0.0

	for _, spec := range specs {
		if spec.value != encoding && spec.value != "*" {
			continue
		}

		if !found || spec.q > bestQ {
			found = true
			bestQ = spec.q
		}
	}

	return bestQ, found
}
