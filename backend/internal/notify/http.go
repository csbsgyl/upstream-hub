package notify

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const notificationHTTPTimeout = 20 * time.Second

func newNotifyHTTPClient() *resty.Client {
	return resty.New().SetTimeout(notificationHTTPTimeout)
}

func normalizeNotifyURL(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", field)
	}
	return raw, nil
}
