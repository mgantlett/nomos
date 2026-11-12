package cli

import (
	"github.com/mgantlett/nomos/substrate/cli/diag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWalkthroughContentAndScrubPipeline(t *testing.T) {
	dir := t.TempDir()
	wtFile := filepath.Join(dir, "walkthrough.md")
	if err := os.WriteFile(wtFile, []byte("Updated file://"+filepath.ToSlash(dir)+"/README.md"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := diag.NormalizeAndScrubWalkthrough(dir, dir, readWalkthroughContent(wtFile))
	if strings.Contains(got, "file://") || !strings.Contains(got, "README.md") {
		t.Fatalf("unexpected scrubbed walkthrough: %s", got)
	}
}
