package podcastindex

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

// signRequest adds the required authentication headers to a Podcast Index API request
func signRequest(req *http.Request, apiKey, apiSecret, userAgent string) {
	// Generate auth time (Unix timestamp)
	authTime := strconv.FormatInt(time.Now().Unix(), 10)

	// Generate auth hash: SHA1(apiKey + apiSecret + authTime)
	authString := apiKey + apiSecret + authTime
	h := sha1.New()
	h.Write([]byte(authString))
	authHash := hex.EncodeToString(h.Sum(nil))

	// Set required headers
	req.Header.Set("X-Auth-Date", authTime)
	req.Header.Set("X-Auth-Key", apiKey)
	req.Header.Set("Authorization", authHash)
	req.Header.Set("User-Agent", userAgent)
}
