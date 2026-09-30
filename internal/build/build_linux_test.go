//go:build linux

package build

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// O buildah daria nofile=1048576 a cada RUN; o forja precisa passar o teto
// atual, senão o RUN falha em ambientes com teto menor (runners do GitHub).
func TestCurrentUlimitsUsesTheCurrentHardLimit(t *testing.T) {
	var r unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &r); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("nofile=%d:%d", r.Max, r.Max)

	got := currentUlimits()
	if r.Max != unix.RLIM_INFINITY && !contains(got, want) {
		t.Errorf("currentUlimits() = %v, queria conter %q", got, want)
	}
	for _, u := range got {
		name, lim, ok := strings.Cut(u, "=")
		soft, hard, ok2 := strings.Cut(lim, ":")
		if !ok || !ok2 || soft != hard || (name != "nofile" && name != "nproc") {
			t.Errorf("formato inesperado para o buildah: %q", u)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
