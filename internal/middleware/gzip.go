package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

var compressibleTypes = []string{
	"application/json",
	"text/html",
}

type gzipWriter struct {
	http.ResponseWriter
	gzipWriter  *gzip.Writer
	wroteHeader bool
	compress    bool
}

func newGzipWriter(w http.ResponseWriter) *gzipWriter {
	return &gzipWriter{ResponseWriter: w}
}

func (w *gzipWriter) shouldCompress() bool {
	contentType := w.Header().Get("Content-Type")
	for _, t := range compressibleTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}

func (w *gzipWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	if w.shouldCompress() {
		w.compress = true
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
		w.gzipWriter = gzip.NewWriter(w.ResponseWriter)
	}

	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(p))
		}
		w.WriteHeader(http.StatusOK)
	}

	if w.compress {
		return w.gzipWriter.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzipWriter) Close() error {
	if w.gzipWriter != nil {
		return w.gzipWriter.Close()
	}
	return nil
}

type gzipReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newGzipReader(r io.ReadCloser) (*gzipReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &gzipReader{r: r, zr: zr}, nil
}

func (r *gzipReader) Read(p []byte) (int, error) {
	return r.zr.Read(p)
}

func (r *gzipReader) Close() error {
	if err := r.zr.Close(); err != nil {
		return err
	}
	return r.r.Close()
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gw := newGzipWriter(w)
			ow = gw
			defer gw.Close()
		}

		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gr, err := newGzipReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = gr
		}

		next.ServeHTTP(ow, r)
	})
}
