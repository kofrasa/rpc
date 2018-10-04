package xml

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"
)

var (
	// a pool of codecs for the client/server. use via the withCodec function
	codecPool = &sync.Pool{
		New: func() interface{} { return newCodec() },
	}
	emptyReader = strings.NewReader("")
)

// Codec reads and writes XML-RPC messages.
type Codec struct {
	rd *xmlReader
	wr *xmlWriter
}

// withCodec acquires a codec from a pool for the callback and release when done.
// The callback function should not hold a reference to the codec when it completes.
func withCodec(f func(*Codec) error) error {
	c := codecPool.Get().(*Codec)
	err := f(c)
	codecPool.Put(c)
	return err
}

// newCodec return an XML-RPC codec for reading/writing requests and responses
func newCodec() *Codec {
	return &Codec{
		rd: newReader(emptyReader),
		wr: newWriter(ioutil.Discard),
	}
}

// writeRequest serialzes and writes an XML-RPC methodCall
func (c *Codec) writeRequest(w io.Writer, method string, params ...interface{}) error {
	return c.writeRPC(w, makeCall(method, params...))
}

// writeResponse serialzes and writes value as valid XML-RPC methodResponse
func (c *Codec) writeResponse(w io.Writer, params interface{}) error {
	return c.writeRPC(w, makeResponse(params))
}

// writeRPC serialize a value as XML-RPC
func (c *Codec) writeRPC(w io.Writer, rpc interface{}) error {
	c.wr.reset(w)
	var err error
	switch v := rpc.(type) {
	case methodCall:
		err = c.wr.writeCall(v)
	case methodResponse:
		err = c.wr.writeResponse(v)
	case rpcValue:
		err = c.wr.writeValue(v)
	default:
		err = c.wr.writeValue(makeValue(rpc))
	}
	c.wr.Flush()
	return err
}

// readRequest deserialize an XML-RPC methodCall into the method and params pointer receivers
func (c *Codec) readRequest(r io.Reader, method *string, params interface{}) error {
	if err := checkPointer(params); err != nil {
		return err
	}

	var call methodCall
	if err := c.readRPC(r, &call); err != nil {
		return err
	}
	if call.Method == "" {
		return InvalidRequest.New("invalid method name '%s'", call.Method)
	}
	*method = call.Method
	return call.rpcParams.writeTo(params)
}

// readResponse deserialize an XML-RPC methodResponse into the params pointer receiver.
// If the response returned a Fault, the error will be of type xmlrpc.Error
func (c *Codec) readResponse(r io.Reader, reply interface{}) error {
	if err := checkPointer(reply); err != nil {
		return err
	}

	var res methodResponse
	if err := c.readRPC(r, &res); err != nil {
		return err
	}

	if !res.Fault.isEmpty() {
		var fault Fault
		if err := res.Fault.writeTo(&fault); err != nil {
			return err
		}
		return fault
	}

	return res.rpcParams.writeTo(reply)
}

// readRPC deserialize a valid XML-RPC input
func (c *Codec) readRPC(r io.Reader, value interface{}) error {
	if err := checkPointer(value); err != nil {
		return err
	}

	c.rd.reset(r)
	var err error
	switch v := value.(type) {
	case *methodCall:
		err = c.rd.readCall(v)
	case *methodResponse:
		err = c.rd.readResponse(v)
	case *rpcValue:
		err = c.rd.readValue(v)
	default:
		var rpc rpcValue
		if err = c.rd.readValue(&rpc); err == nil || err == io.EOF {
			err = rpc.writeTo(value)
		}
	}

	if err == nil || err == io.EOF {
		return nil
	}

	if v, ok := err.(*xml.SyntaxError); ok {
		return MalformedInput.New(v.Error())
	}

	return err
}

/// Helper methods ///

// checkPointer validates that the value is a pointer type
func checkPointer(v interface{}) error {
	refPtrKind := reflect.TypeOf(v).Kind()
	if refPtrKind != reflect.Ptr {
		return InternalError.New("error decoding to value. expected pointer got '%s'", refPtrKind)
	}
	return nil
}
