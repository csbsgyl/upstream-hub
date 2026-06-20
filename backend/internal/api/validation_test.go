package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/config"
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

func TestBalanceTrendRangeFromLegacyDays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		url  string
		want string
	}{
		{name: "default", url: "/", want: "24h"},
		{name: "one day", url: "/?days=1", want: "24h"},
		{name: "week", url: "/?days=7", want: "7d"},
		{name: "month", url: "/?days=30", want: "30d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", tc.url, nil)
			if got := balanceTrendRangeFromLegacyDays(c); got != tc.want {
				t.Fatalf("balanceTrendRangeFromLegacyDays() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSafeBackupName(t *testing.T) {
	valid, err := safeBackupName("upstream-hub-20260618-120000.sql.gz")
	if err != nil || valid != "upstream-hub-20260618-120000.sql.gz" {
		t.Fatalf("safeBackupName valid = %q, %v", valid, err)
	}

	for _, raw := range []string{"../secret.sql.gz", `..\secret.sql.gz`, "latest.json", "upstream.env", "", "nested/backup.sql.gz"} {
		if _, err := safeBackupName(raw); err == nil {
			t.Fatalf("safeBackupName(%q) returned nil error, want rejection", raw)
		}
	}
}

func TestDBDSN(t *testing.T) {
	got := dbDSN(config.DatabaseConfig{
		Host:    "postgres",
		Port:    5432,
		User:    "upstreamhub",
		Name:    "upstream hub",
		SSLMode: "disable",
	})
	if !strings.HasPrefix(got, "postgresql://upstreamhub@postgres:5432/upstream%20hub?") {
		t.Fatalf("dbDSN = %q, want encoded host/path", got)
	}
	if !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("dbDSN = %q, want sslmode", got)
	}
}

func TestParseUpdateStatusTracksLivePhase(t *testing.T) {
	status := parseUpdateStatus([]string{
		"[UPSTREAMHUB_STAGE] check|Checking Docker and Compose",
		"[UPSTREAMHUB_STAGE] build|Building image and restarting service",
	})
	if status.Status != "running" || !status.Running || status.Phase != "build" || status.Progress != 76 {
		t.Fatalf("parseUpdateStatus() = %#v, want running build phase", status)
	}
	if status.Message == "" {
		t.Fatalf("parseUpdateStatus message is empty, want phase message")
	}
}

func TestParseUpdateStatusTerminalStates(t *testing.T) {
	completed := parseUpdateStatus([]string{
		"[UPSTREAMHUB_STAGE] health|Waiting for health check",
		"[UPSTREAMHUB_STAGE] done|Deployment completed",
	})
	if completed.Status != "completed" || !completed.Completed || completed.Running || completed.Progress != 100 {
		t.Fatalf("completed status = %#v, want completed 100%%", completed)
	}

	failed := parseUpdateStatus([]string{
		"[UPSTREAMHUB_STAGE] build|Building image and restarting service",
		"[UPSTREAMHUB_STAGE] failed|Update failed with exit code 1",
	})
	if failed.Status != "failed" || !failed.Failed || failed.Running || failed.Phase != "failed" {
		t.Fatalf("failed status = %#v, want failed terminal state", failed)
	}
}

func TestDockerUpdateArgsMountsHostRepoAndSocket(t *testing.T) {
	args := dockerUpdateArgs("/srv/upstream-hub", "/var/run/docker.sock", "upstream-hub:local", "backups/update.log")
	joined := strings.Join(args, "\n")
	for _, want := range []string{
		"run",
		"-d",
		"--name\nupstreamhub-updater",
		"type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
		"type=bind,source=/srv/upstream-hub,target=/srv/upstream-hub",
		"upstream-hub:local",
		"bash ./scripts/deploy.sh",
		"[UPSTREAMHUB_STAGE] start|Updater container started",
		"[UPSTREAMHUB_STAGE] done|Update completed",
		"[UPSTREAMHUB_STAGE] failed|Update failed with exit code",
		"backups/update.log",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker args %q do not contain %q", joined, want)
		}
	}
}

func TestCleanDockerPath(t *testing.T) {
	got := cleanDockerPath(`C:\srv\upstream-hub`)
	want := filepath.Clean("C:/srv/upstream-hub")
	if got != want {
		t.Fatalf("cleanDockerPath() = %q, want %q", got, want)
	}
}

func TestUpdateEnabledEnv(t *testing.T) {
	t.Setenv("UPSTREAMHUB_UPDATE_ENABLED", "false")
	if updateEnabled() {
		t.Fatal("updateEnabled() = true, want false")
	}
	t.Setenv("UPSTREAMHUB_UPDATE_ENABLED", "true")
	if !updateEnabled() {
		t.Fatal("updateEnabled() = false, want true")
	}
}
