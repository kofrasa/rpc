# rpc/xml

[![build status](https://img.shields.io/travis/kofrasa/rpc.svg)](http://travis-ci.org/kofrasa/rpc) [![GoDoc](https://godoc.org/github.com/kofrasa/rpc?status.svg)](https://godoc.org/github.com/kofrasa/rpc)

XML-RPC codec for Go compatible with [gorilla/rpc](https://github.com/gorilla/rpc)

## install

latest

```sh
go get github.com/kofrasa/rpc
```

versions

```sh
go get gopkg.in/kofrasa/rpc.v1
```

## docs

[godoc](https://godoc.org/github.com/kofrasa/rpc)

## usage

### server

```go

import (
  "net/http"
  "github.com/gorilla/rpc/v2"
  "github.com/kofrasa/rpc/xml"
)

type Args struct {
  A, B int
}

type Reply struct {
  C int
}

type PositionalArgs []int

type Arith int

func (t *Arith) Add(r *http.Request, args *Args, reply *Reply) error {
  reply.C = args.A + args.B
  return nil
}

func (t *Arith) Max(r *http.Request, args *PositionalArgs, reply *Reply) error {
  nums := *args
  reply.C = num[0]
  for i := 1; i < len(num); i++ {
    if reply.C < num[i] {
      reply.C = num[i]
    }
  }
  return nil
}

func main() {
  s := rpc.NewServer()
  s.RegisterCodec(xml.NewServerCodec(), "text/xml")
  s.RegisterService(new(Arith), "Arith")
  http.ListenAndServe("http://localhost:5000", s)
}
```

### client

```go
import (
  "fmt"
  "net/http"
  "time"
  "github.com/kofrasa/rpc/xml"
)

type Args struct {
  A, B int
}

type Reply struct {
  C int
}

const addr = "http://localhost:5000"

func main() {
  client := xml.NewClient(addr)

  args := Args{A: 3, B: 3}
  var reply Reply

  // encodes args as a single struct parameter
  client.Call("Arith.Add", &reply, args)
  fmt.Printf("%d + %d = %d\n", args.A, args.B, reply.C) // expect 6

  // encodes variable arguments as multiple parameters
  client.Call("Arith.Max", &reply, 2, 5, 3)
  fmt.Printf("Max(%d,%d,%d) = %d\n", 2, 5, 3, reply.C) // expect 5

  // you can also create a client with your own custom httpclient
  customHTTPClient := &http.Client{Timeout: time.Second * 5}
  customRPCClient := xml.NewClient(addr, xml.WithHTTPClient(customHTTPClient))
}

```

## features

* Extended [iso8601](https://en.wikipedia.org/wiki/ISO_8601) formats.
  * `20060102T15:04:05`
  * `2006-01-02T15:04:05`
  * `2006-01-02T15:04:05-07:00`
  * `2006-01-02T15:04:05Z07:00`
* Decodes boolean `true` and `false`
* Server method aliases
* Server accept encoding for `gzip` and `deflate`
* Custom `"rpc"` tag for translating struct field names

## license

MIT
