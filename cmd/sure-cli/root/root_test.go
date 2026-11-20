package root

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/config"
)

func TestTimeoutFlagSetsTheClientTimeout(t *testing.T) {
	cli := New()
	var got time.Duration
	cli.AddCommand(&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) { got = config.Timeout() }})
	cli.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "none.yaml"), "--timeout", "2m", "probe"})
	if err := cli.Execute(); err != nil {
		t.Fatal(err)
	}
	if got != 2*time.Minute {
		t.Fatalf("timeout = %v, want 2m", got)
	}
}
