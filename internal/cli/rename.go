package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/laurenschristian/eerox/internal/eero"
)

// renameBatchCmd applies nicknames from a file: one "match<TAB>new name" per line,
// where match is an id, ip, mac, nickname or hostname. "#" starts a comment.
func renameBatchCmd() *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "rename-batch <file>",
		Short: "Rename many devices from a file of match<TAB>name lines (--dry-run to preview)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			devs, err := client.Devices(ctx, id)
			if err != nil {
				return err
			}
			done, same, failed := 0, 0, 0
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				match, name, ok := strings.Cut(line, "\t")
				if !ok {
					match, name, ok = strings.Cut(line, "=")
				}
				name = strings.TrimSpace(name)
				match = strings.TrimSpace(match)
				if !ok || name == "" {
					fmt.Fprintf(os.Stderr, "skip malformed line: %s\n", line)
					failed++
					continue
				}
				d, err := eero.FindDevice(devs, match)
				if err != nil {
					fmt.Fprintf(os.Stderr, "skip: %v\n", err)
					failed++
					continue
				}
				if d.Nickname == name {
					same++
					continue
				}
				fmt.Printf("%-40s -> %s\n", d.Name(), name)
				if dryRun {
					d.Nickname = name
					done++
					continue
				}
				if err := client.UpdateDevice(ctx, id, d.ID(), map[string]any{"nickname": name}); err != nil {
					fmt.Fprintf(os.Stderr, "failed %s: %v\n", d.Name(), err)
					failed++
					continue
				}
				d.Nickname = name
				done++
			}
			if err := sc.Err(); err != nil {
				return err
			}
			mode := ""
			if dryRun {
				mode = " (dry run)"
			}
			fmt.Printf("renamed %d, unchanged %d, skipped %d%s\n", done, same, failed, mode)
			return nil
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan, change nothing")
	return c
}
