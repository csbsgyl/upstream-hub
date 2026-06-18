package version

import "strings"

var (
	Name       = "upstream-hub"
	Version    = "0.1.0-dev"
	Commit     = "dev"
	Branch     = "main"
	BuildTime  = ""
	Repository = "csbsgyl/upstream-hub"
)

type Info struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"short_commit"`
	Branch      string `json:"branch"`
	BuildTime   string `json:"build_time,omitempty"`
	Repository  string `json:"repository"`
	RepoURL     string `json:"repo_url"`
	CommitKnown bool   `json:"commit_known"`
}

func Current() Info {
	repo := normalizeRepository(Repository)
	return Info{
		Name:        Name,
		Version:     Version,
		Commit:      strings.TrimSpace(Commit),
		ShortCommit: ShortSHA(Commit),
		Branch:      strings.TrimSpace(Branch),
		BuildTime:   strings.TrimSpace(BuildTime),
		Repository:  repo,
		RepoURL:     RepoURL(repo),
		CommitKnown: CommitKnown(Commit),
	}
}

func RepoURL(repo string) string {
	repo = normalizeRepository(repo)
	if repo == "" {
		repo = normalizeRepository(Repository)
	}
	return "https://github.com/" + repo
}

func ShortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if !CommitKnown(sha) {
		return "unknown"
	}
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func CommitKnown(sha string) bool {
	sha = strings.TrimSpace(strings.ToLower(sha))
	return sha != "" && sha != "dev" && sha != "unknown" && sha != "none"
}

func DifferentCommit(current, latest string) bool {
	current = strings.TrimSpace(strings.ToLower(current))
	latest = strings.TrimSpace(strings.ToLower(latest))
	if !CommitKnown(current) || !CommitKnown(latest) {
		return false
	}
	return current != latest && !strings.HasPrefix(current, latest) && !strings.HasPrefix(latest, current)
}

func normalizeRepository(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	return strings.Trim(repo, "/")
}
