package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/pktline"
	"github.com/spf13/cobra"
)

// loadFilterKey resolves the repository key once per filter process. It is a
// package variable so tests can inject a fixed key without a real repository.
var loadFilterKey = loadRepoKey

// filterProcessCmd is Git's long-running clean/smudge filter. A single process
// handles every managed file for a git operation, so the key is resolved once
// instead of shelling out to `git config` and re-reading it per file.
var filterProcessCmd = &cobra.Command{
	Use:    "filter-process",
	Short:  "Git long-running clean/smudge filter",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFilterProcess(os.Stdin, os.Stdout)
	},
}

func runFilterProcess(stdin io.Reader, stdout io.Writer) error {
	out := bufio.NewWriter(stdout)
	r := pktline.NewReader(stdin)
	w := pktline.NewWriter(out)

	if err := filterHandshake(r, w, out); err != nil {
		return err
	}

	var (
		key       *crypto.Key
		keyErr    error
		keyLoaded bool
	)
	defer func() { key.Destroy() }()

	for {
		meta, err := r.ReadTextLinesUntilFlush()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(meta) == 0 {
			return nil
		}
		command := metaValue(meta, "command")
		pathname := metaValue(meta, "pathname")
		content, err := r.ReadDataUntilFlush()
		if err != nil {
			return err
		}

		if !keyLoaded {
			key, keyErr = loadFilterKey()
			keyLoaded = true
		}

		var result []byte
		switch command {
		case "clean":
			if keyErr != nil {
				err = keyErr
			} else {
				result, err = envfile.Encrypt(content, key)
			}
		case "smudge":
			// Smudge failures pass the encrypted content through so the git
			// operation succeeds; clean failures above stay fatal (fail closed).
			if keyErr != nil {
				result = smudgeFallback(pathname, keyErr, content)
			} else if result, err = envfile.Decrypt(content, key); err != nil {
				result, err = smudgeFallback(pathname, err, content), nil
			}
		default:
			err = fmt.Errorf("unsupported filter command %q", command)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "envapor: %v\n", err)
			if werr := writeFilterStatus(w, out, "error"); werr != nil {
				return werr
			}
			continue
		}
		if err := writeFilterResult(w, out, result); err != nil {
			return err
		}
	}
}

// filterHandshake performs the version-2 handshake. It flushes the underlying
// writer after our version reply so Git proceeds to send its capability list
// (Git blocks on our reply, so buffering it would deadlock).
func filterHandshake(r *pktline.Reader, w *pktline.Writer, out *bufio.Writer) error {
	intro, err := r.ReadTextLinesUntilFlush()
	if err != nil {
		return err
	}
	if !containsLine(intro, "git-filter-client") || !containsLine(intro, "version=2") {
		return fmt.Errorf("envapor: unexpected filter handshake %q", intro)
	}
	if err := w.WriteText("git-filter-server\n"); err != nil {
		return err
	}
	if err := w.WriteText("version=2\n"); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := out.Flush(); err != nil {
		return err
	}

	if _, err := r.ReadTextLinesUntilFlush(); err != nil {
		return err
	}
	for _, c := range []string{"capability=clean\n", "capability=smudge\n"} {
		if err := w.WriteText(c); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return out.Flush()
}

func writeFilterStatus(w *pktline.Writer, out *bufio.Writer, status string) error {
	if err := w.WriteText("status=" + status + "\n"); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return out.Flush()
}

func writeFilterResult(w *pktline.Writer, out *bufio.Writer, result []byte) error {
	if err := w.WriteText("status=success\n"); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := w.WriteData(result); err != nil {
		return err
	}
	// First flush ends the content list; the second ends the (empty) trailing
	// status list that would otherwise report errors during content transfer.
	if err := w.Flush(); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return out.Flush()
}

func metaValue(lines []string, key string) string {
	prefix := key + "="
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return l[len(prefix):]
		}
	}
	return ""
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(filterProcessCmd)
}
