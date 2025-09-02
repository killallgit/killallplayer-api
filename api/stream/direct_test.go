package stream

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStreamDirectURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryURL       string
		setupServer    func() *httptest.Server
		expectedStatus int
		expectedError  string
		rangeHeader    string
	}{
		{
			name:           "Missing URL parameter",
			queryURL:       "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "URL parameter is required",
		},
		{
			name:           "Invalid URL format",
			queryURL:       "not a valid url",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid URL format",
		},
		{
			name:           "Non-HTTP/HTTPS scheme",
			queryURL:       "ftp://example.com/audio.mp3",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Only HTTP and HTTPS URLs are allowed",
		},
		{
			name:           "Private network address",
			queryURL:       "http://192.168.1.1/audio.mp3",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Access to private networks is not allowed",
		},
		{
			name:           "Localhost address",
			queryURL:       "http://localhost/audio.mp3",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Access to private networks is not allowed",
		},
		{
			name:           "Empty host",
			queryURL:       "http:///audio.mp3",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "URL must have a valid host",
		},
		{
			name:     "URL too long",
			queryURL: "http://example.com/" + string(make([]byte, MaxURLLength)),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "URL is too long",
		},
		{
			name:     "Successful streaming",
			queryURL: "http://test-server/audio.mp3",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "audio/mpeg")
					w.Header().Set("Content-Length", "1000")
					w.WriteHeader(http.StatusOK)
					data := make([]byte, 1000)
					w.Write(data)
				}))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "Range request",
			queryURL: "http://test-server/audio.mp3",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Range") == "bytes=0-499" {
						w.Header().Set("Content-Type", "audio/mpeg")
						w.Header().Set("Content-Range", "bytes 0-499/1000")
						w.Header().Set("Content-Length", "500")
						w.WriteHeader(http.StatusPartialContent)
						data := make([]byte, 500)
						w.Write(data)
					} else {
						w.WriteHeader(http.StatusBadRequest)
					}
				}))
			},
			rangeHeader:    "bytes=0-499",
			expectedStatus: http.StatusPartialContent,
		},
		{
			name:     "Source returns error",
			queryURL: "http://test-server/notfound.mp3",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectedStatus: http.StatusBadGateway,
			expectedError:  "Audio source returned error: 404",
		},
		{
			name:     "HTML content instead of audio",
			queryURL: "http://test-server/page.html",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("<html><body>Not audio</body></html>"))
				}))
			},
			expectedStatus: http.StatusBadGateway,
			expectedError:  "Audio source returned HTML instead of audio content",
		},
		{
			name:     "Headers properly copied",
			queryURL: "http://test-server/audio.mp3",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "audio/mpeg")
					w.Header().Set("Content-Length", "1000")
					w.Header().Set("Accept-Ranges", "bytes")
					w.Header().Set("ETag", "\"abc123\"")
					w.Header().Set("Last-Modified", "Wed, 15 Nov 1995 04:58:08 GMT")
					w.Header().Set("Cache-Control", "max-age=3600")
					w.WriteHeader(http.StatusOK)
					data := make([]byte, 1000)
					w.Write(data)
				}))
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server if needed
			var testServer *httptest.Server
			queryURL := tt.queryURL
			if tt.setupServer != nil {
				testServer = tt.setupServer()
				defer testServer.Close()
				
				// Replace test-server with actual server URL
				if queryURL != "" {
					queryURL = testServer.URL + "/audio.mp3"
				}
			}

			// Create test context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			// Build request URL with query parameter
			reqURL := "/stream/direct"
			if queryURL != "" {
				reqURL += "?url=" + url.QueryEscape(queryURL)
			}
			
			c.Request = httptest.NewRequest("GET", reqURL, nil)
			
			if tt.rangeHeader != "" {
				c.Request.Header.Set("Range", tt.rangeHeader)
			}

			// Execute handler
			handler := StreamDirectURL()
			handler(c)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}

			// Check headers were copied for successful requests
			if tt.name == "Headers properly copied" && tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
				assert.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
				assert.Equal(t, "\"abc123\"", w.Header().Get("ETag"))
				assert.Equal(t, "Wed, 15 Nov 1995 04:58:08 GMT", w.Header().Get("Last-Modified"))
				assert.Equal(t, "max-age=3600", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestDirectStreamingWithFlusher(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test server that sends data slowly
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		
		// Send data in chunks with delays to test flushing
		for i := 0; i < 5; i++ {
			data := bytes.Repeat([]byte{byte(i)}, 100)
			w.Write(data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer testServer.Close()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	reqURL := fmt.Sprintf("/stream/direct?url=%s", url.QueryEscape(testServer.URL))
	c.Request = httptest.NewRequest("GET", reqURL, nil)

	// Execute handler
	handler := StreamDirectURL()
	handler(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	// Should have received 500 bytes (5 chunks of 100 bytes)
	assert.Equal(t, 500, w.Body.Len())
}

func TestDirectStreamClientDisconnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test server that sends a lot of data
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		
		// Try to send a lot of data
		for i := 0; i < 1000; i++ {
			data := make([]byte, 1024)
			_, err := w.Write(data)
			if err != nil {
				// Client disconnected
				return
			}
		}
	}))
	defer testServer.Close()

	// Create test context with context that will be cancelled
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	reqURL := fmt.Sprintf("/stream/direct?url=%s", url.QueryEscape(testServer.URL))
	c.Request = httptest.NewRequest("GET", reqURL, nil).WithContext(ctx)

	// Execute handler
	handler := StreamDirectURL()
	handler(c)

	// The handler should handle the disconnection gracefully
	// We don't check specific status as the client disconnected
}

func TestPrivateAddressDetection(t *testing.T) {
	tests := []struct {
		hostname string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"::1", true},
		{"fe80::1", true},
		{"example.com", false},
		{"8.8.8.8", false},
		{"google.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := isPrivateOrLocalAddress(tt.hostname)
			assert.Equal(t, tt.expected, result, "For hostname: %s", tt.hostname)
		})
	}
}

func TestRedirectHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	redirectCount := 0
	// Create a chain of redirects
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirectCount < 3 {
			redirectCount++
			http.Redirect(w, r, fmt.Sprintf("/redirect%d", redirectCount), http.StatusFound)
			return
		}
		// Final response
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio data"))
	}))
	defer testServer.Close()

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	reqURL := fmt.Sprintf("/stream/direct?url=%s", url.QueryEscape(testServer.URL))
	c.Request = httptest.NewRequest("GET", reqURL, nil)

	// Execute handler
	handler := StreamDirectURL()
	handler(c)

	// Should successfully stream after following redirects
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "audio data")
}