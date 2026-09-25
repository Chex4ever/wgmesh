// Package cli — команды wgmesh (Cobra).
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/i18n"
)

// cfgPath — путь к главному конфигу, флаг --config.
var cfgPath string

// langFlag — флаг языка локализации --lang (auto, en, ru).
var langFlag string

// Execute — точка входа CLI.
func Execute() {
	cobra.MousetrapHelpText = ""

	root := &cobra.Command{
		Use:   "wgmesh",
		Short: "wgmesh (Warp Gateway Mesh) — менеджер WireGuard mesh-сетей (multihop exit-маршруты)",
		Long: `wgmesh управляет одноранговой mesh-сетью из WireGuard-нод
(Linux / Mikrotik / OpenWRT): YAML-конфиги в Git, маршруты через
несколько хопов, один бинарник без центрального сервера.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if langFlag != "" {
				i18n.SetLanguage(langFlag)
			} else {
				if m, err := config.Load(cfgPath); err == nil && m.Language != "" {
					i18n.SetLanguage(m.Language)
				} else {
					i18n.SetLanguage("auto")
				}
			}
			return nil
		},
		RunE: runTUICmd,
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", config.DefaultConfigFile,
		"путь к главному YAML-конфигу")
	root.PersistentFlags().StringVar(&langFlag, "lang", "",
		"язык интерфейса / UI language (auto, en, ru)")

	root.AddCommand(
		initCmd(),
		nodeCmd(),
		routeCmd(),
		planCmd(),
		statusCmd(),
		exportCmd(),
		gitCmd(),
		applyCmd(),
		clientConfigCmd(),
		tuiCmd(),
		doctorCmd(),
		versionCmd(),
		updateCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Stderr.WriteString("Ошибка: " + err.Error() + "\n")
		os.Exit(1)
	}
}

// Version — заполняется через -ldflags при сборке.
var Version = "dev"

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Показать версию",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("wgmesh %s\n", Version)
		},
	}
}

// loadMesh — утилита загрузки конфига для команд.
func loadMesh() (*config.Mesh, error) {
	return config.Load(cfgPath)
}

// saveMesh — утилита сохранения.
func saveMesh(m *config.Mesh) error {
	return config.Save(cfgPath, m)
}

