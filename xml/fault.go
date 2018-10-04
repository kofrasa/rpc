package xml

import (
	"fmt"
	"strconv"
)

// Fault represents an XML-RPC fault.
type Fault struct {
	Code    int    `rpc:"faultCode"`
	Message string `rpc:"faultString"`
}

// Error returns a formatted error string
func (f Fault) Error() string {
	return fmt.Sprintf("%d: %s", f.Code, f.Message)
}

type faultCode int

// Codes: http://xmlrpc-epi.sourceforge.net/specs/rfc.fault_codes.php
const (
	// parse error
	MalformedInput      faultCode = -32700
	UnsupportedEncoding faultCode = -32701
	InvalidCharacter    faultCode = -32702
	// server error
	InvalidRequest faultCode = -32600
	MethodNotFound faultCode = -32601
	InvalidParams  faultCode = -32602
	InternalError  faultCode = -32603
)

var (
	faultMessages = map[faultCode]string{
		MalformedInput:      "malformed input",
		UnsupportedEncoding: "unsupported encoding",
		InvalidCharacter:    "invalid character for encoding",
		InvalidRequest:      "invalid xml-rpc. not conforming to spec",
		MethodNotFound:      "requested method not found",
		InvalidParams:       "invalid method parameters",
		InternalError:       "internal xml-rpc error",
	}
)

func (f faultCode) String() string {
	return faultMessages[f]
}

func (f faultCode) Error() string {
	return strconv.Itoa(int(f)) + ": " + f.String()
}

func (f faultCode) New(format string, v ...interface{}) Fault {
	s := fmt.Sprintf(format, v...)
	if len(s) == 0 {
		s = f.String()
	}
	return Fault{Code: int(f), Message: s}
}
