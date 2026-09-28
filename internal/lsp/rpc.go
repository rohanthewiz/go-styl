// Package lsp is a Language Server Protocol server for Stylus, built on the
// go-styl compiler: diagnostics as you type, completion, hover with computed
// values, go-to-definition across @import, document symbols, color swatches
// and formatting (styl fmt).
//
// The protocol layer is hand-rolled rather than taken from an LSP library:
// the server needs a dozen message types, and the module's only dependency
// is serr, which keeps `go install …/cmd/styl-lsp` a single small static
// binary with nothing else to pull.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

// message is any JSON-RPC 2.0 message. A request has an ID and a Method, a
// notification only a Method, and a response an ID with Result or Error.
// ID stays raw JSON because clients may send numbers or strings and the
// response must echo the same form.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC / LSP error codes used by the server.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeRequestFailed  = -32803
)

// conn reads and writes LSP base-protocol frames: a header block
// ("Content-Length: N\r\n\r\n") followed by N bytes of JSON.
type conn struct {
	r  *bufio.Reader
	mu sync.Mutex // serializes writes; responses and notifications interleave
	w  io.Writer
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

// read returns the next message. io.EOF means the client closed the stream.
func (c *conn) read() (*message, error) {
	// textproto handles the MIME-style header block, including the
	// optional Content-Type header some clients send.
	hdr, err := textproto.NewReader(c.r).ReadMIMEHeader()
	if err != nil {
		if err == io.EOF || strings.Contains(err.Error(), "EOF") {
			return nil, io.EOF
		}
		return nil, err
	}
	n, err := strconv.Atoi(hdr.Get("Content-Length"))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("lsp: bad Content-Length %q", hdr.Get("Content-Length"))
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, &rpcError{Code: codeParseError, Message: err.Error()}
	}
	return &m, nil
}

func (e *rpcError) Error() string { return e.Message }

// write sends one message.
func (c *conn) write(m *message) error {
	m.JSONRPC = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

// reply answers request id with result, or with err when it is non-nil.
// A nil result is sent as JSON null: LSP requires the result member on
// success, and omitempty would drop it.
func (c *conn) reply(id json.RawMessage, result any, err *rpcError) error {
	if err != nil {
		return c.write(&message{ID: id, Error: err})
	}
	if result == nil {
		result = json.RawMessage("null")
	}
	return c.write(&message{ID: id, Result: result})
}

// notify sends a server → client notification.
func (c *conn) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(&message{Method: method, Params: raw})
}
