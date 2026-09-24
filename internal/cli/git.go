package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func gitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Коллаборация через Git (init, pull, push, status, log)",
	}
	cmd.AddCommand(gitInitCmd(), gitPullCmd(), gitPushCmd(), gitStatusCmd(), gitLogCmd())
	return cmd
}

func gitInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Инициализировать Git репозиторий для конфигурации",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runGit("init"); err != nil {
				return err
			}
			gitignore := "*.private.yaml\nknown_hosts\n.meshctl-*.yaml\n"
			if err := os.WriteFile(".gitignore", []byte(gitignore), 0o644); err != nil {
				return err
			}
			if err := runGit("add", "mesh.yaml", ".gitignore"); err != nil {
				return err
			}
			if err := runGit("commit", "-m", "initial meshctl config"); err != nil {
				return err
			}
			fmt.Println("✔ Git репозиторий инициализирован, секреты добавлены в .gitignore")
			return nil
		},
	}
}

func gitPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Получить изменения из удалённого репозитория (git pull)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGit("pull")
		},
	}
}

func gitPushCmd() *cobra.Command {
	var msg string
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Отправить изменения в удалённый репозиторий (git push)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if msg == "" {
				msg = "update meshctl config"
			}
			runGit("add", "mesh.yaml")
			runGit("commit", "-m", msg)
			return runGit("push")
		},
	}
	cmd.Flags().StringVarP(&msg, "message", "m", "", "сообщение коммита")
	return cmd
}

func gitStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Показать статус конфигурации в Git",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGit("status", "-s")
		},
	}
}

func gitLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "История изменений конфигурации",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGit("log", "-n", "10", "--oneline")
		},
	}
}

func runGit(args ...string) error {
	c := exec.Command("git", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
