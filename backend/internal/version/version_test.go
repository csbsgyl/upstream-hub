package version

import "testing"

func TestShortSHA(t *testing.T) {
	if got := ShortSHA("cb0bec2fc78ad4e961a432cf69794ff4a928f1a8"); got != "cb0bec2" {
		t.Fatalf("ShortSHA = %q, want cb0bec2", got)
	}
	if got := ShortSHA("dev"); got != "unknown" {
		t.Fatalf("ShortSHA(dev) = %q, want unknown", got)
	}
}

func TestDifferentCommit(t *testing.T) {
	if !DifferentCommit("cb0bec2fc78ad4e961a432cf69794ff4a928f1a8", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("DifferentCommit returned false for different full commits")
	}
	if DifferentCommit("cb0bec2", "cb0bec2fc78ad4e961a432cf69794ff4a928f1a8") {
		t.Fatal("DifferentCommit returned true for matching short/full commit")
	}
	if DifferentCommit("dev", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("DifferentCommit returned true when current commit is unknown")
	}
}

func TestCurrentNormalizesRepository(t *testing.T) {
	oldRepo := Repository
	Repository = "https://github.com/csbsgyl/upstream-hub.git"
	defer func() { Repository = oldRepo }()

	got := Current()
	if got.Repository != "csbsgyl/upstream-hub" {
		t.Fatalf("Repository = %q, want csbsgyl/upstream-hub", got.Repository)
	}
	if got.RepoURL != "https://github.com/csbsgyl/upstream-hub" {
		t.Fatalf("RepoURL = %q, want GitHub URL", got.RepoURL)
	}
}
