package xml

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

type xmlTag int

const (
	valueTag          xmlTag = iota
	structTag         xmlTag = iota
	arrayTag          xmlTag = iota
	dataTag           xmlTag = iota
	base64Tag         xmlTag = iota
	booleanTag        xmlTag = iota
	dateTimeTag       xmlTag = iota
	doubleTag         xmlTag = iota
	intTag            xmlTag = iota
	stringTag         xmlTag = iota
	memberTag         xmlTag = iota
	nameTag           xmlTag = iota
	methodCallTag     xmlTag = iota
	methodNameTag     xmlTag = iota
	methodResponseTag xmlTag = iota
	paramListTag      xmlTag = iota
	paramTag          xmlTag = iota
	faultTag          xmlTag = iota
)

var (
	tagNames = map[xmlTag]string{
		valueTag:          "value",
		structTag:         "struct",
		arrayTag:          "array",
		dataTag:           "data",
		base64Tag:         "base64",
		booleanTag:        "boolean",
		dateTimeTag:       "dateTime.iso8601",
		doubleTag:         "double",
		intTag:            "int",
		stringTag:         "string",
		memberTag:         "member",
		nameTag:           "name",
		methodCallTag:     "methodCall",
		methodNameTag:     "methodName",
		methodResponseTag: "methodResponse",
		paramListTag:      "params",
		paramTag:          "param",
		faultTag:          "fault",
	}
	startTags     [18]string
	endTags       [18]string
	boolEncodeMap = map[bool]string{true: "1", false: "0"}
)

type flusher interface {
	Flush() error
}

func init() {
	// precreate start and end tags
	for t, n := range tagNames {
		startTags[t] = "<" + n + ">"
		endTags[t] = "</" + n + ">"
	}
}

// writes XML-RPC values to an io.Writer
type xmlWriter struct {
	wr io.Writer
}

func newWriter(w io.Writer) *xmlWriter {
	return &xmlWriter{wr: w}
}

func (w *xmlWriter) reset(wr io.Writer) {
	w.Flush()
	w.wr = wr
}

func (w *xmlWriter) Flush() error {
	if f, ok := w.wr.(flusher); ok {
		return f.Flush()
	}
	return nil
}

// writeRaw write the given raw value enclosed in the specified tag
func (w *xmlWriter) writeRaw(t xmlTag, raw string) error {
	if _, err := io.WriteString(w.wr, startTags[t]); err != nil {
		return err
	}
	if _, err := io.WriteString(w.wr, raw); err != nil {
		return err
	}
	_, err := io.WriteString(w.wr, endTags[t])
	return err
}

// writeXML invokes the given function wrapped in the specified tag
func (w *xmlWriter) writeXML(t xmlTag, fn func() error) error {
	if _, err := io.WriteString(w.wr, startTags[t]); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	_, err := io.WriteString(w.wr, endTags[t])
	return err
}

func (w *xmlWriter) writeCall(rpc methodCall) error {
	if _, err := io.WriteString(w.wr, xml.Header); err != nil {
		return err
	}
	return w.writeXML(methodCallTag, func() error {
		if err := w.writeRaw(methodNameTag, rpc.Method); err != nil {
			return err
		}
		return w.writeXML(paramListTag, func() error {
			for _, v := range rpc.Params {
				err := w.writeXML(paramTag, func() error {
					return w.writeValue(v)
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (w *xmlWriter) writeResponse(rpc methodResponse) error {
	if _, err := io.WriteString(w.wr, xml.Header); err != nil {
		return err
	}
	return w.writeXML(methodResponseTag, func() error {
		if !rpc.Fault.isEmpty() {
			return w.writeXML(faultTag, func() error {
				return w.writeValue(rpc.Fault)
			})
		}
		return w.writeXML(paramListTag, func() error {
			for _, v := range rpc.Params {
				err := w.writeXML(paramTag, func() error {
					return w.writeValue(v)
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (w *xmlWriter) writeValue(rpc rpcValue) error {
	return w.writeXML(valueTag, func() error {
		switch rpc.kind {
		case intKind:
			return w.writeRaw(intTag, fmt.Sprint(rpc.value))
		case booleanKind:
			return w.writeRaw(booleanTag, boolEncodeMap[rpc.value.(bool)])
		case doubleKind:
			d := fmt.Sprintf("%f", rpc.value)
			d = strings.TrimRight(d, "0")
			if len(d) == 0 || d[len(d)-1] == '.' {
				d = d + "0"
			}
			return w.writeRaw(doubleTag, d)
		case stringKind:
			s := rpc.value.(string)
			if strings.IndexAny(s, `<>&'"`) == -1 {
				return w.writeRaw(stringTag, s)
			}
			return w.writeXML(stringTag, func() error {
				return xml.EscapeText(w.wr, []byte(s))
			})
		case dateTimeKind:
			t := rpc.value.(time.Time)
			var a [64]byte
			b := a[:0]
			return w.writeRaw(dateTimeTag, string(t.AppendFormat(b, iso8601)))
		case base64Kind:
			return w.writeRaw(base64Tag, base64.StdEncoding.EncodeToString(rpc.value.([]byte)))
		case arrayKind:
			return w.writeXML(arrayTag, func() error {
				return w.writeXML(dataTag, func() error {
					array := rpc.value.([]rpcValue)
					for _, v := range array {
						if err := w.writeValue(v); err != nil {
							return err
						}
					}
					return nil
				})
			})
		case structKind:
			return w.writeXML(structTag, func() error {
				members := rpc.value.([]rpcEntry)
				for _, m := range members {
					err := w.writeXML(memberTag, func() error {
						if err := w.writeRaw(nameTag, m.Name); err != nil {
							return err
						}
						return w.writeValue(m.Value)
					})
					if err != nil {
						return err
					}
				}
				return nil
			})
		default:
			return nil
		}
	})
}
