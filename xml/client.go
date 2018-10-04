package xml

import (
	"bytes"
	"net/http"
	"sync"
)

// A Client is used to make XML-RPC calls.
type Client struct {
	url        string
	username   string
	password   string
	client     *http.Client
	header     http.Header
	bufPoolMap map[string]*sync.Pool
	bufMtx     sync.Mutex
}

// NewClient returns a new XML-RPC client.
func NewClient(url string, options ...func(*Client)) *Client {
	c := &Client{
		url:        url,
		bufPoolMap: make(map[string]*sync.Pool),
		client:     http.DefaultClient,
		header:     make(http.Header),
	}

	for _, opt := range options {
		opt(c)
	}

	c.header.Set("Content-Type", "text/xml")

	return c
}

// WithBasicAuth configure client with basic HTTP authentication.
func WithBasicAuth(username, password string) func(*Client) {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// WithHTTPClient confgure a custom HTTP client to use for connecting to server.
func WithHTTPClient(httpClient *http.Client) func(*Client) {
	return func(c *Client) {
		c.client = httpClient
	}
}

// WithHTTPHeader configure headers to add to each request.
func WithHTTPHeader(header http.Header) func(*Client) {
	return func(c *Client) {
		for k, v := range header {
			for _, w := range v {
				c.header.Set(k, w)
			}
		}
	}
}

// Call sends an XML-RPC request to the server.
// If a non-nil error is returned, it may be an rpc.Fault or some other type of error
func (c *Client) Call(method string, reply interface{}, args ...interface{}) error {
	return withCodec(func(codec *Codec) error {
		return c.withBuffer(method, func(buf *bytes.Buffer) error {
			if err := codec.writeRequest(buf, method, args...); err != nil {
				return err
			}

			req, err := http.NewRequest("POST", c.url, buf)
			if err != nil {
				return err
			}

			// set custom request headers
			req.Header = c.header

			if c.username != "" && c.password != "" {
				req.SetBasicAuth(c.username, c.password)
			}

			resp, err := c.client.Do(req)
			if err != nil {
				return err
			}

			dec := newDecompressor(resp)
			err = codec.readResponse(dec, reply)
			dec.Close()
			return err
		})
	})
}

func (c *Client) withBuffer(method string, fn func(*bytes.Buffer) error) error {
	c.bufMtx.Lock()
	pool, ok := c.bufPoolMap[method]
	if !ok {
		pool = &sync.Pool{
			New: func() interface{} { return bytes.NewBuffer([]byte{}) },
		}
		c.bufPoolMap[method] = pool
	}
	c.bufMtx.Unlock()

	buf := pool.Get().(*bytes.Buffer)
	err := fn(buf)
	buf.Reset()
	pool.Put(buf)
	return err
}
