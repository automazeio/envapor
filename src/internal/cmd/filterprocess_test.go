package cmd

import (
	"bytes"
	"testing"

	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/pktline"
)

func TestFilterProcessCleanAndSmudge(t *testing.T) {
	key, err := crypto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	orig := loadFilterKey
	loadFilterKey = func() (*crypto.Key, error) { return key, nil }
	defer func() { loadFilterKey = orig }()

	plaintext := []byte("SECRET=hunter2\nAPP_ENV=production # PUBLIC\n")
	ciphertext, err := envfile.Encrypt(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}

	var in bytes.Buffer
	w := pktline.NewWriter(&in)
	writeHandshake(t, w)
	writeCommand(t, w, "clean", plaintext)
	writeCommand(t, w, "smudge", ciphertext)

	var out bytes.Buffer
	if err := runFilterProcess(&in, &out); err != nil {
		t.Fatalf("runFilterProcess: %v", err)
	}

	r := pktline.NewReader(&out)
	readHandshake(t, r)

	if got := readCommandResult(t, r); !bytes.Equal(got, ciphertext) {
		t.Fatalf("clean output = %q, want ciphertext %q", got, ciphertext)
	}
	if got := readCommandResult(t, r); !bytes.Equal(got, plaintext) {
		t.Fatalf("smudge output = %q, want plaintext %q", got, plaintext)
	}
}

func TestFilterProcessKeyErrorFailsClosed(t *testing.T) {
	orig := loadFilterKey
	loadFilterKey = func() (*crypto.Key, error) { return nil, errNotInitialized }
	defer func() { loadFilterKey = orig }()

	var in bytes.Buffer
	w := pktline.NewWriter(&in)
	writeHandshake(t, w)
	writeCommand(t, w, "clean", []byte("SECRET=hunter2\n"))

	var out bytes.Buffer
	if err := runFilterProcess(&in, &out); err != nil {
		t.Fatalf("runFilterProcess: %v", err)
	}

	r := pktline.NewReader(&out)
	readHandshake(t, r)
	status, err := r.ReadTextLinesUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	if !containsLine(status, "status=error") {
		t.Fatalf("status = %v, want status=error", status)
	}
}

func TestFilterProcessSmudgeWrongKeyPassesThrough(t *testing.T) {
	encKey, err := crypto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := crypto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	orig := loadFilterKey
	loadFilterKey = func() (*crypto.Key, error) { return wrongKey, nil }
	defer func() { loadFilterKey = orig }()

	ciphertext, err := envfile.Encrypt([]byte("SECRET=hunter2\n"), encKey)
	if err != nil {
		t.Fatal(err)
	}

	var in bytes.Buffer
	w := pktline.NewWriter(&in)
	writeHandshake(t, w)
	writeCommand(t, w, "smudge", ciphertext)

	var out bytes.Buffer
	if err := runFilterProcess(&in, &out); err != nil {
		t.Fatalf("runFilterProcess: %v", err)
	}

	r := pktline.NewReader(&out)
	readHandshake(t, r)
	if got := readCommandResult(t, r); !bytes.Equal(got, ciphertext) {
		t.Fatalf("smudge with wrong key = %q, want encrypted pass-through %q", got, ciphertext)
	}
}

func writeHandshake(t *testing.T, w *pktline.Writer) {
	t.Helper()
	must(t, w.WriteText("git-filter-client\n"))
	must(t, w.WriteText("version=2\n"))
	must(t, w.Flush())
	must(t, w.WriteText("capability=clean\n"))
	must(t, w.WriteText("capability=smudge\n"))
	must(t, w.Flush())
}

func writeCommand(t *testing.T, w *pktline.Writer, command string, content []byte) {
	t.Helper()
	must(t, w.WriteText("command="+command+"\n"))
	must(t, w.WriteText("pathname=.env\n"))
	must(t, w.Flush())
	must(t, w.WriteData(content))
	must(t, w.Flush())
}

func readHandshake(t *testing.T, r *pktline.Reader) {
	t.Helper()
	server, err := r.ReadTextLinesUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	if !containsLine(server, "git-filter-server") || !containsLine(server, "version=2") {
		t.Fatalf("server handshake = %v", server)
	}
	if _, err := r.ReadTextLinesUntilFlush(); err != nil {
		t.Fatal(err)
	}
}

func readCommandResult(t *testing.T, r *pktline.Reader) []byte {
	t.Helper()
	status, err := r.ReadTextLinesUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	if !containsLine(status, "status=success") {
		t.Fatalf("status = %v, want status=success", status)
	}
	data, err := r.ReadDataUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	trailing, err := r.ReadTextLinesUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	if len(trailing) != 0 {
		t.Fatalf("trailing status = %v, want empty", trailing)
	}
	return data
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
