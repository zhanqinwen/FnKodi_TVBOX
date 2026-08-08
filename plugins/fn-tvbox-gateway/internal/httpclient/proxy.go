package httpclient

import (
	"net"
	"net/http"
	"time"
)

// NewProxy returns an HTTP client for media proxying.
// It MUST NOT set Client.Timeout (that would cut long bodies).
// Only Transport.ResponseHeaderTimeout is set (TTFB / response headers).
func NewProxy(headerTimeout time.Duration, userAgent string) *http.Client {
	if headerTimeout <= 0 {
		headerTimeout = 15 * time.Second
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: headerTimeout,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		// intentionally no Timeout
		Transport: &uaTransport{base: transport, ua: userAgent},
	}
}
