package awsv2

import (
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// bodyShieldClient wraps the SDK's HTTP client so the request body that
// net/http sees exposes only Read and Close.
//
// Why: smithy-go builds a request body whose type implements io.WriterTo
// (safeWriteToReadCloser) when the payload is an in-memory reader, and it
// closes that body as soon as the response has arrived. Once closed, its
// WriteTo returns (0, io.EOF) while Read returns (0, io.EOF). net/http's
// persistConn.writeLoop finishes every request with a second copy of the body
// into io.Discard (to detect bodies longer than Content-Length); io.Copy uses
// WriterTo when it is available and passes the io.EOF error returned by the
// closed body straight through, so the write loop treats a perfectly good request
// as failed and closes the connection. If the response body is still being read
// (a response bigger than the buffered 4 KB, a fast local server, or a loaded
// machine) that read then fails with "use of closed network connection".
// Hiding WriterTo makes io.Copy use Read, where io.EOF is the normal end of
// stream, and removes the race. The SDK still closes the original body.
type bodyShieldClient struct {
	next aws.HTTPClient
}

// shieldHTTPClient returns c wrapped in a bodyShieldClient (nil stays nil).
func shieldHTTPClient(c aws.HTTPClient) aws.HTTPClient {
	if c == nil {
		return nil
	}
	if _, ok := c.(*bodyShieldClient); ok {
		return c
	}
	return &bodyShieldClient{next: c}
}

func (c *bodyShieldClient) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil && req.Body != http.NoBody {
		req.Body = readCloseOnly{req.Body}
	}
	return c.next.Do(req)
}

// readCloseOnly hides every optional interface (notably io.WriterTo) of the
// wrapped body.
type readCloseOnly struct {
	rc io.ReadCloser
}

func (b readCloseOnly) Read(p []byte) (int, error) { return b.rc.Read(p) }
func (b readCloseOnly) Close() error               { return b.rc.Close() }
