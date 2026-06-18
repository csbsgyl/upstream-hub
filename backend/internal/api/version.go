package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	appversion "github.com/worryzyy/upstream-hub/internal/version"
)

type versionCheckResponse struct {
	Current       appversion.Info  `json:"current"`
	LatestCommit  string           `json:"latest_commit,omitempty"`
	LatestShort   string           `json:"latest_short,omitempty"`
	LatestHTMLURL string           `json:"latest_html_url,omitempty"`
	CompareURL    string           `json:"compare_url,omitempty"`
	UpdateURL     string           `json:"update_url"`
	UpdateCommand string           `json:"update_command"`
	AutoUpdate    updateCapability `json:"auto_update"`
	HasUpdate     bool             `json:"has_update"`
	CheckError    string           `json:"check_error,omitempty"`
	CheckedAt     time.Time        `json:"checked_at"`
}

type cachedVersionCheck struct {
	latest    githubCommitResponse
	expiresAt time.Time
}

var (
	versionCheckMu    sync.Mutex
	versionCheckCache = map[string]cachedVersionCheck{}
)

func registerVersion(g *gin.RouterGroup, d *Deps) {
	g.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, appversion.Current())
	})
	g.GET("/version/check", func(c *gin.Context) { checkVersion(c, d) })
}

func checkVersion(c *gin.Context, d *Deps) {
	current := appversion.Current()
	force := c.Query("force") == "1" || c.Query("force") == "true"
	resp := versionCheckResponse{
		Current:       current,
		UpdateURL:     current.RepoURL,
		UpdateCommand: "git pull && ./scripts/deploy.sh",
		AutoUpdate:    detectUpdateCapability(d),
		CheckedAt:     time.Now(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	latest, err := fetchLatestGitHubCommit(ctx, current.Repository, current.Branch, force)
	if err != nil {
		resp.CheckError = err.Error()
		c.JSON(http.StatusOK, gin.H{"data": resp})
		return
	}

	resp.LatestCommit = latest.SHA
	resp.LatestShort = appversion.ShortSHA(latest.SHA)
	resp.LatestHTMLURL = latest.HTMLURL
	resp.HasUpdate = appversion.DifferentCommit(current.Commit, latest.SHA)
	if resp.HasUpdate && current.CommitKnown {
		resp.CompareURL = fmt.Sprintf("%s/compare/%s...%s", current.RepoURL, current.Commit, latest.SHA)
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

type githubCommitResponse struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
}

func fetchLatestGitHubCommit(ctx context.Context, repo, branch string, force bool) (*githubCommitResponse, error) {
	if repo == "" {
		repo = appversion.Current().Repository
	}
	if branch == "" {
		branch = "main"
	}
	cacheKey := repo + "@" + branch
	now := time.Now()

	if !force {
		versionCheckMu.Lock()
		if cached, ok := versionCheckCache[cacheKey]; ok && cached.expiresAt.After(now) {
			latest := cached.latest
			versionCheckMu.Unlock()
			return &latest, nil
		}
		versionCheckMu.Unlock()
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "upstream-hub-update-check")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("github returned HTTP %d", res.StatusCode)
	}

	var out githubCommitResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.SHA == "" {
		return nil, fmt.Errorf("github response missing sha")
	}
	if out.HTMLURL == "" {
		out.HTMLURL = fmt.Sprintf("https://github.com/%s/commit/%s", repo, out.SHA)
	}
	versionCheckMu.Lock()
	versionCheckCache[cacheKey] = cachedVersionCheck{
		latest:    out,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	versionCheckMu.Unlock()
	return &out, nil
}
