package xml

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

var (
	largeXML                 = createXML(1e6, "Allan Watt")
	largeXMLQuoted           = createXML(1e6, "Allan&#39;s Watt")
	largeRPC, largeRPCQuoted rpcValue
)

func createXML(count int, text string) string {
	var s bytes.Buffer
	s.WriteString("<value><array><data>")
	for i := 0; i < count; i++ {
		s.WriteString(fmt.Sprint("<value><string>", text, " ", i+1, "</string></value>"))
	}
	s.WriteString("</data></array></value>")
	return s.String()
}

func Benchmark_Reader(b *testing.B) {
	buf := strings.NewReader(largeXML)
	p := newReader(buf)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		largeRPC.value = nil
		largeRPC.kind = nilKind
		buf.Seek(0, io.SeekStart)
		p.readValue(&largeRPC)
	}
}

func Benchmark_ReaderQuoted(b *testing.B) {
	buf := strings.NewReader(largeXMLQuoted)
	p := newReader(buf)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		largeRPCQuoted.value = nil
		largeRPCQuoted.kind = nilKind
		buf.Seek(0, io.SeekStart)
		p.readValue(&largeRPCQuoted)
	}
}

func Benchmark_Writer(b *testing.B) {
	buf := bytes.NewBufferString("")
	w := newWriter(buf)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		w.writeValue(largeRPC)
	}
}

func Benchmark_WriterQuoted(b *testing.B) {
	buf := bytes.NewBufferString("")
	w := newWriter(buf)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		w.writeValue(largeRPCQuoted)
	}
}
