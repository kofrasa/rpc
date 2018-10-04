package xml

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"
)

var (
	contentEncodingRe = regexp.MustCompile(`(gzip|deflate)`)
	gzipWriterPool    = &sync.Pool{
		New: func() interface{} { return gzip.NewWriter(ioutil.Discard) },
	}
	flateWriterPool = &sync.Pool{
		New: func() interface{} { w, _ := flate.NewWriter(ioutil.Discard, flate.DefaultCompression); return w },
	}
)

type writeResetter interface {
	io.WriteCloser
	Reset(io.Writer)
}

type compressWriter struct {
	writeResetter
	encoding string
}

func (w *compressWriter) Close() error {
	err := w.writeResetter.Close()
	switch w.encoding {
	case "gzip":
		gzipWriterPool.Put(w.writeResetter)
	case "deflate":
		flateWriterPool.Put(w.writeResetter)
	}
	return err
}

func newCompressor(w http.ResponseWriter, header http.Header) io.Writer {
	encoding := header.Get("Accept-Encoding")
	if encoding != "" {
		encoding = contentEncodingRe.FindString(encoding)
	}
	switch encoding {
	case "gzip":
		w.Header().Set("Content-Encoding", "gzip")
		zw := &compressWriter{writeResetter: gzipWriterPool.Get().(*gzip.Writer), encoding: encoding}
		zw.Reset(w)
		return zw
	case "deflate":
		w.Header().Set("Content-Encoding", "deflate")
		zw := &compressWriter{writeResetter: flateWriterPool.Get().(*flate.Writer), encoding: encoding}
		zw.Reset(w)
		return zw
	default:
		return w
	}
}

func newDecompressor(resp *http.Response) io.ReadCloser {
	encoding := resp.Header.Get("Content-Encoding")
	if encoding != "" {
		encoding = contentEncodingRe.FindString(encoding)
	}
	switch encoding {
	case "gzip":
		zr, _ := gzip.NewReader(resp.Body)
		return zr
	case "deflate":
		return flate.NewReader(resp.Body)
	}
	return resp.Body
}
