package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/notify"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func TestNormalizeSubscriptionsRejectsEmptyGroups(t *testing.T) {
	_, err := normalizeSubscriptions(`[{"channel_id":1,"mode":"groups","groups":[]}]`)
	if err == nil || !strings.Contains(err.Error(), "at least one group") {
		t.Fatalf("normalizeSubscriptions error = %v, want empty groups rejection", err)
	}
}

func TestNormalizeSubscriptionsDefaultsModeAll(t *testing.T) {
	got, err := normalizeSubscriptions(`[{"channel_id":1,"groups":["ignored"]}]`)
	if err != nil {
		t.Fatalf("normalizeSubscriptions error = %v", err)
	}
	var list []notify.Subscription
	if err := json.Unmarshal([]byte(got), &list); err != nil {
		t.Fatalf("normalized subscriptions invalid JSON: %v", err)
	}
	if len(list) != 1 || list[0].ChannelID != 1 || list[0].Mode != notify.SubscriptionModeAll || len(list[0].Groups) != 0 {
		t.Fatalf("normalized subscriptions = %#v, want channel 1 mode all with no groups", list)
	}
}

func TestNormalizeSubscriptionsTrimsAndDedupesGroups(t *testing.T) {
	got, err := normalizeSubscriptions(`[{"channel_id":1,"mode":"groups","groups":[" gpt-4 ","gpt-4","","claude"]}]`)
	if err != nil {
		t.Fatalf("normalizeSubscriptions error = %v", err)
	}
	var list []notify.Subscription
	if err := json.Unmarshal([]byte(got), &list); err != nil {
		t.Fatalf("normalized subscriptions invalid JSON: %v", err)
	}
	if len(list) != 1 || len(list[0].Groups) != 2 || list[0].Groups[0] != "gpt-4" || list[0].Groups[1] != "claude" {
		t.Fatalf("normalized groups = %#v, want trimmed unique groups", list)
	}
}

func TestNormalizeSubscriptionsRejectsMissingChannelID(t *testing.T) {
	_, err := normalizeSubscriptions(`[{"mode":"all"}]`)
	if err == nil || !strings.Contains(err.Error(), "channel_id") {
		t.Fatalf("normalizeSubscriptions error = %v, want channel_id rejection", err)
	}
}

func TestNormalizeSubscriptionsRejectsUnknownMode(t *testing.T) {
	_, err := normalizeSubscriptions(`[{"channel_id":1,"mode":"none"}]`)
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("normalizeSubscriptions error = %v, want mode rejection", err)
	}
}

func TestValidateNotifyConfigRejectsBadURL(t *testing.T) {
	err := validateNotifyConfig(storage.NotifyWebhook, `{"url":"http://%"}`)
	if err == nil {
		t.Fatal("validateNotifyConfig returned nil, want invalid URL error")
	}
}

func TestValidateNotifyConfigAcceptsServerChan(t *testing.T) {
	err := validateNotifyConfig(storage.NotifyServerChan, `{"sendkey":"SCT123"}`)
	if err != nil {
		t.Fatalf("validateNotifyConfig error = %v", err)
	}
}

func TestValidateCaptchaConfigRejectsUnknownProvider(t *testing.T) {
	err := validateCaptchaConfig(captchaInput{Name: "main", Type: "unknown"}, "key")
	if err == nil || !strings.Contains(err.Error(), "unknown captcha provider") {
		t.Fatalf("validateCaptchaConfig error = %v, want unknown provider rejection", err)
	}
}

func TestValidateCaptchaConfigAcceptsKnownProvider(t *testing.T) {
	err := validateCaptchaConfig(captchaInput{Name: "main", Type: storage.CaptchaTwoCaptcha}, "key")
	if err != nil {
		t.Fatalf("validateCaptchaConfig error = %v", err)
	}
}

func TestValidateCaptchaConfigRejectsEmptyAPIKey(t *testing.T) {
	err := validateCaptchaConfig(captchaInput{Name: "main", Type: storage.CaptchaTwoCaptcha}, " ")
	if err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("validateCaptchaConfig error = %v, want api_key rejection", err)
	}
}

func TestNormalizeOptionalHTTPURL(t *testing.T) {
	got, err := normalizeOptionalHTTPURL(" https://example.com/base/ ", "endpoint")
	if err != nil {
		t.Fatalf("normalizeOptionalHTTPURL error = %v", err)
	}
	if got != "https://example.com/base" {
		t.Fatalf("normalizeOptionalHTTPURL = %q, want trimmed URL without trailing slash", got)
	}
	_, err = normalizeOptionalHTTPURL("ftp://example.com", "endpoint")
	if err == nil || !strings.Contains(err.Error(), "http") {
		t.Fatalf("normalizeOptionalHTTPURL error = %v, want http(s) rejection", err)
	}
}

func TestQueryIntClamped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?limit=9999&bad=abc&zero=0", nil)

	if got := queryIntClamped(c, "limit", 100, 1, 500); got != 500 {
		t.Fatalf("limit clamp = %d, want 500", got)
	}
	if got := queryIntClamped(c, "bad", 100, 1, 500); got != 100 {
		t.Fatalf("bad default = %d, want 100", got)
	}
	if got := queryIntClamped(c, "zero", 100, 1, 500); got != 1 {
		t.Fatalf("zero clamp = %d, want 1", got)
	}
}
