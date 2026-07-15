package gitutil

import "testing"

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/a b'$`\\tool")
	want := "'/tmp/a b'\"'\"'$`\\tool'"
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}
