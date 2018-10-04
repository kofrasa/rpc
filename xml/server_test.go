package xml

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"testing"

	"github.com/gorilla/rpc/v2"
)

type PositionalArgs []interface{}
type NumericArgs []int

type Args struct {
	A, B int
}

type Reply struct {
	C int
}

type Arith int

func (t *Arith) Add(r *http.Request, args *Args, reply *Reply) error {
	reply.C = args.A + args.B
	return nil
}

func (t *Arith) Mul(r *http.Request, args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Div(r *http.Request, args *Args, reply *Reply) error {
	if args.B == 0 {
		return InvalidParams.New("divide by zero")
	}
	reply.C = args.A / args.B
	return nil
}

func (t *Arith) Max(r *http.Request, args *NumericArgs, reply *Reply) error {
	params := *args
	if len(params) == 0 {
		reply.C = 0
		return nil
	}

	reply.C = params[0]
	for i := 1; i < len(params); i++ {
		if reply.C < params[i] {
			reply.C = params[i]
		}
	}
	return nil
}

func (t *Arith) Count(r *http.Request, args *PositionalArgs, reply *Reply) error {
	params := *args
	reply.C = len(params)
	return nil
}

func createConn() (*http.Server, *Client) {
	address := "127.0.0.1:5000"
	codec := NewServerCodec()
	codec.RegisterAlias("add", "Add")

	s := rpc.NewServer()
	s.RegisterCodec(codec, "text/xml")
	s.RegisterService(new(Arith), "Arith")

	header := make(http.Header)
	header.Set("Accept-Encoding", "deflate")
	client := NewClient(fmt.Sprintf("http://%s", address), WithHTTPHeader(header))
	return &http.Server{Addr: address, Handler: s}, client
}

func Test_ClientServer(t *testing.T) {
	server, c := createConn()
	defer server.Shutdown(context.Background())
	go server.ListenAndServe()
	runtime.Gosched()

	args := Args{A: 3, B: 3}
	var reply Reply

	err := c.Call("Arith.Add", &reply, args)
	assertEqual(t, nil, err, "Add no error")
	assertEqual(t, 6, reply.C, "Add")

	reply.C = -1
	c.Call("Arith.add", &reply, args)
	assertEqual(t, 6, reply.C, "add lowercase alias")

	c.Call("Arith.Mul", &reply, args)
	assertEqual(t, 9, reply.C, "Mul")

	c.Call("Arith.Div", &reply, args)
	assertEqual(t, 1, reply.C, "Div")

	reply.C = -1
	args = Args{A: 1, B: 0}
	err = c.Call("Arith.Div", &reply, args)
	assertNotEqual(t, nil, err, "error on divide by zero")
	assertEqual(t, -1, reply.C, "fail divide by zero")

	fault, ok := err.(Fault)
	assertOk(t, ok, "expect fault")
	assertEqual(t, int(InvalidParams), fault.Code, "read fault code")
	assertEqual(t, "divide by zero", fault.Message, "read fault string")

	reply.C = -1
	c.Call("Arith.Max", &reply, 5, 9, 7, 5)
	assertEqual(t, 9, reply.C, "Max with multiple homogeneous values")

	reply.C = -1
	c.Call("Arith.Count", &reply, 5, "hello", 7.0, []int{1})
	assertEqual(t, 4, reply.C, "Count with multiple heterogeneous values")

	reply.C = -1
	err = c.Call("Arith.Factorize", &reply, args)
	fault, ok = err.(Fault)
	assertNotEqual(t, nil, err, "error for unknown method")
	assertEqual(t, int(MethodNotFound), fault.Code, "method not found")
}
