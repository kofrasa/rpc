package xml

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	iso8601         = "20060102T15:04:05"
	rfc3339NoTZ     = "2006-01-02T15:04:05"
	rfc3339HyphenTZ = "2006-01-02T15:04:05-07:00"
)

var (
	dateTimeFormats = [4]string{iso8601, time.RFC3339, rfc3339HyphenTZ, rfc3339NoTZ}
	boolDecodeMap   = map[string]bool{"1": true, "true": true, "0": false, "false": false}
	valueTagSet     = map[string]bool{}
)

// reads an XML-RPC input from an io.Reader
type xmlReader struct {
	dec  *xml.Decoder // for XML pull parsing
	peek xml.Token    // next token we peeked
}

func init() {
	for _, t := range [8]xmlTag{stringTag, intTag, base64Tag, dateTimeTag, doubleTag, booleanTag, arrayTag, structTag} {
		valueTagSet[tagNames[t]] = true
	}
	valueTagSet["i4"] = true //alternative for int tags
}

func newReader(r io.Reader) *xmlReader {
	return &xmlReader{
		dec: xml.NewDecoder(r),
	}
}

// resets the reader internal state
func (r *xmlReader) reset(rd io.Reader) {
	r.peek = nil
	r.dec = xml.NewDecoder(rd)
}

func (r *xmlReader) readHeader() error {
	r.trim()
	t, err := r.token()
	if err != nil {
		return err
	}
	if hdr, ok := t.(xml.ProcInst); ok {
		if hdr.Target == "xml" {
			return nil
		}
	}
	r.putToken(t)
	return nil
}

func (r *xmlReader) readCall(rpc *methodCall) error {
	if err := r.readHeader(); err != nil {
		return err
	}

	err := r.expectStart("methodCall")
	if err != nil {
		return err
	}

	if err = r.expectStart("methodName"); err != nil {
		return err
	}
	if rpc.Method, err = r.nextText(); err != nil {
		return err
	}
	if err = r.expectEnd("methodName"); err != nil {
		return err
	}

	if err = r.readParams(&rpc.rpcParams); err != nil {
		return err
	}

	return r.expectEnd("methodCall")
}

func (r *xmlReader) readResponse(rpc *methodResponse) error {
	if err := r.readHeader(); err != nil {
		return err
	}

	err := r.expectStart("methodResponse")
	if err != nil {
		return err
	}

	if err = r.readParams(&rpc.rpcParams); err != nil && err != io.EOF {
		if err = r.expectStart("fault"); err != nil {
			return err
		}
		if err = r.readValue(&rpc.Fault); err != nil {
			return err
		}
		if err = r.expectEnd("fault"); err != nil {
			return err
		}
	}
	return r.expectEnd("methodResponse")
}

func (r *xmlReader) readParams(rpc *rpcParams) error {
	err := r.expectStart("params")
	if err != nil {
		return err
	}

	for {
		err = r.expectStart("param")
		if err != nil {
			// we assume we hit the end
			break
		}

		var v rpcValue
		if err = r.readValue(&v); err != nil {
			return err
		}
		rpc.Params = append(rpc.Params, v)

		err = r.expectEnd("param")
		if err != nil {
			return err
		}
	}

	return r.expectEnd("params")
}

// readValue decodes and reads the next value
func (r *xmlReader) readValue(rpc *rpcValue) error {
	err := r.expectStart("value")
	if err != nil {
		return err
	}

	// determine the type of value
	se, err := r.nextStart()
	if err != nil {

		// empty value or unwrapped string
		s, err := r.nextText()
		if err != nil {
			return r.expectEnd("value")
		}
		// treat empty value as empty string
		rpc.value = s
		rpc.kind = stringKind
		return err
	}

	if !valueTagSet[se.Name.Local] {
		return fmt.Errorf("parsing error. expected valid rpc value element got '%s'", se.Name.Local)
	}

	r.putToken(se)

	switch se.Name.Local {
	case "array":
		err = r.readArray(rpc)
	case "struct":
		err = r.readStruct(rpc)
	default:
		err = r.readPrimitive(rpc)
	}

	if err != nil {
		return err
	}

	// match end tag
	return r.expectEnd("value")
}

// readPrimitive reads the next primitive value
func (r *xmlReader) readPrimitive(rpc *rpcValue) error {
	// assume start is valid since we always come via readValue()
	se, err := r.nextStart()
	if err != nil {
		return err
	}

	s, _ := r.nextText()
	if err = r.expectEnd(se.Name.Local); err != nil {
		return err
	}

	var ok bool

	switch se.Name.Local {
	case "string":
		rpc.value = s
		rpc.kind = stringKind
	case "boolean":
		if rpc.value, ok = boolDecodeMap[s]; !ok {
			return InvalidRequest.New("error writing boolean '%s'", s)
		}
		rpc.kind = booleanKind
	case "int", "i4":
		if rpc.value, err = strconv.Atoi(s); err != nil {
			return InvalidRequest.New("error writing int '%s'", s)
		}
		rpc.kind = intKind
	case "double":
		if rpc.value, err = strconv.ParseFloat(s, 64); err != nil {
			return InvalidRequest.New("error writing double '%s'", s)
		}
		rpc.kind = doubleKind
	case "base64":
		rpc.value, err = base64.StdEncoding.DecodeString(s)
		rpc.kind = base64Kind
	case "dateTime.iso8601":
		for _, dateFmt := range dateTimeFormats {
			if rpc.value, err = time.Parse(dateFmt, s); err == nil {
				break
			}
		}
		rpc.kind = dateTimeKind
	default:
		return fmt.Errorf("unhandled tag. '%s'", se.Name.Local)
	}
	return err
}

// readArray reads an array value
func (r *xmlReader) readArray(rpc *rpcValue) error {
	r.nextStart() // <array>

	// <data>
	err := r.expectStart("data")
	if err != nil {
		return err
	}

	var array []rpcValue

	for {
		se, err := r.nextStart()
		if err != nil {
			// empty array is allowed although just a waste of bytes
			break
		}

		// we expect every start element to be a value
		if se.Name.Local != "value" {
			return fmt.Errorf("parsing error. invalid element '%s'", se.Name.Local)
		}

		// read the values
		var val rpcValue

		r.putToken(se)
		if err := r.readValue(&val); err != nil {
			return err
		}

		array = append(array, val)
	}

	rpc.value = array
	rpc.kind = arrayKind

	err = r.expectEnd("data")
	if err != nil {
		return err
	}

	return r.expectEnd("array")
}

// readStruct reads the struct value
func (r *xmlReader) readStruct(rpc *rpcValue) error {
	r.nextStart() // <struct>

	var members []rpcEntry

	for {
		err := r.expectStart("member")
		if err != nil {
			// empty structs are allowed
			break
		}

		// read the member details
		var entry rpcEntry

		// start member
		if err = r.expectStart("name"); err != nil {
			return err
		}
		if entry.Name, err = r.nextText(); err != nil {
			return err
		}
		if err = r.expectEnd("name"); err != nil {
			return err
		}

		if err = r.readValue(&entry.Value); err != nil {
			return err
		}

		if err = r.expectEnd("member"); err != nil {
			return err
		}

		members = append(members, entry)
	}

	rpc.value = members
	rpc.kind = structKind

	// we expect to be at the end of the struct
	return r.expectEnd("struct")
}

// nextText read the required next token as text. treat empty text as an error
func (r *xmlReader) nextText() (string, error) {
	t, err := r.token()
	if t == nil {
		return "", err
	}
	if cd, ok := t.(xml.CharData); ok {
		return string(cd), nil
	}
	r.putToken(t)
	return "", fmt.Errorf("expected chardata but got '%#v'", t)
}

// nextStart return the next token expected as an xml.StartElement
func (r *xmlReader) nextStart() (xml.StartElement, error) {
	r.trim()
	t, err := r.token()
	if t == nil {
		return xml.StartElement{}, err
	}
	if se, ok := t.(xml.StartElement); ok {
		return se, nil
	}
	r.putToken(t)
	return xml.StartElement{}, fmt.Errorf("expected start element but got '%s'", t)
}

// nextEnd return the next token expected as an xml.EndElement
func (r *xmlReader) nextEnd() (xml.EndElement, error) {
	r.trim()
	t, err := r.token()
	if t == nil {
		return xml.EndElement{}, err
	}
	if end, ok := t.(xml.EndElement); ok {
		return end, nil
	}
	r.putToken(t)
	return xml.EndElement{}, fmt.Errorf("expected end element but got '%s'", t)
}

// expect a start element with the given name
func (r *xmlReader) expectStart(name string) error {
	se, err := r.nextStart()
	if err != nil {
		return err
	}
	if se.Name.Local != name {
		r.putToken(se)
		return fmt.Errorf("parsing error. expected start element '%s' but got '%s'", name, se.Name.Local)
	}
	return nil
}

// expect an end element with the given name
func (r *xmlReader) expectEnd(name string) error {
	end, err := r.nextEnd()
	if err != nil {
		return err
	}
	if end.Name.Local != name {
		r.putToken(end)
		return fmt.Errorf("parsing error. expected end element '%s' but got '%s'", name, end.Name.Local)
	}
	return nil
}

// token returns the next token from the XML stream
func (r *xmlReader) token() (xml.Token, error) {
	if r.peek != nil {
		t := r.peek
		r.peek = nil
		return t, nil
	}
	return r.dec.RawToken()
}

func (r *xmlReader) trim() {
	for {
		t, _ := r.token()
		if cd, ok := t.(xml.CharData); ok {
			if strings.TrimSpace(string(cd)) == "" {
				continue
			}
		}
		r.putToken(t)
		break
	}
}

// pushes the token on the top of decoding stream
func (r *xmlReader) putToken(t xml.Token) {
	r.peek = t
}
