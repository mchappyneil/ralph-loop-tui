package main

// httpReporter implements Reporter by POSTing events to a ralph-hub server.
// This is a stub — Task 4 replaces it with retry logic and a sender goroutine.
type httpReporter struct {
	hubURL string
	apiKey string
}

// newHTTPReporter creates an httpReporter that sends events to hubURL.
func newHTTPReporter(hubURL, apiKey, _, _ string) *httpReporter {
	return &httpReporter{
		hubURL: hubURL,
		apiKey: apiKey,
	}
}

func (h *httpReporter) Send(Event) {}

func (h *httpReporter) Close() error { return nil }
