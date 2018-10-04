package xml

import (
	"net/http"
	"strings"

	"github.com/gorilla/rpc/v2"
)

const (
	// gorilla error strings
	methodNotFound  = "rpc: can't find method"
	serviceNotFound = "rpc: can't find service"
)

// ServerCodec codec compatible with gorilla/rpc to process each request.
type ServerCodec struct {
	aliases map[string]string
}

// serverRequest handles reading request and writing response
type serverRequest struct {
	header http.Header
	call   methodCall
	err    error
}

// NewServerCodec return a new XML-RPC severCodec compatible with "gorilla/rpc".
func NewServerCodec() *ServerCodec {
	return &ServerCodec{aliases: make(map[string]string)}
}

// RegisterAlias register a method alias.
func (c *ServerCodec) RegisterAlias(alias, method string) {
	c.aliases[alias] = method
}

// NewRequest returns a new codec request.
func (c *ServerCodec) NewRequest(r *http.Request) rpc.CodecRequest {
	s := &serverRequest{header: r.Header}

	s.err = withCodec(func(c *Codec) error {
		return c.readRPC(r.Body, &s.call)
	})

	// resolve aliases
	parts := strings.Split(s.call.Method, ".")
	if len(parts) == 2 {
		if method, ok := c.aliases[parts[1]]; ok {
			parts[1] = method
			s.call.Method = strings.Join(parts, ".")
		}
	}

	return s
}

// Method reads the XML-RPC request and returns the method name.
func (s *serverRequest) Method() (string, error) {
	return s.call.Method, s.err
}

// ReadRequest reads the XML-RPC request and writes the arguments to the receiver.
func (s *serverRequest) ReadRequest(args interface{}) error {
	return s.call.rpcParams.writeTo(args)
}

// WriteResponse write an XML-RPC response to reply receiver.
func (s *serverRequest) WriteResponse(w http.ResponseWriter, reply interface{}) {
	withCodec(func(c *Codec) error {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		zw := newCompressor(w, s.header)
		c.writeResponse(zw, reply)
		if closer, _ := zw.(*compressWriter); closer != nil {
			closer.Close()
		}
		return nil
	})
}

// WriteError write an XML-RPC Fault.
func (s *serverRequest) WriteError(w http.ResponseWriter, status int, err error) {
	// XML-RPC always send 200 OK responses
	switch v := err.(type) {
	case Fault:
		s.WriteResponse(w, v)
	default:
		if strings.HasPrefix(err.Error(), methodNotFound) || strings.HasPrefix(v.Error(), serviceNotFound) {
			s.WriteResponse(w, MethodNotFound.New(""))
		} else {
			// service functions should return appropriate XML-RPC faults
			// wrap any other error as internal
			s.WriteResponse(w, InternalError.New(err.Error()))
		}
	}
}
