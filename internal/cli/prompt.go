package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// confirmPrompt запрашивает интерактивное подтверждение y/N.
func confirmPrompt(cmd *cobra.Command, promptStr string) (bool, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s (y/N): ", promptStr)
	var resp string
	_, err := fmt.Fscanln(cmd.InOrStdin(), &resp)
	if err != nil && err.Error() != "unexpected newline" {
		return false, nil
	}
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes", nil
}

// expandHome разворачивает prefix ~/ в путь к домашней директории пользователя.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
