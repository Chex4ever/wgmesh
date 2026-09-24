// Package cli — команды meshctl (Cobra).
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
)

// cfgPath — путь к главному конфигу, флаг --config.
var cfgPath string

// Execute — точка входа CLI.
func Execute() {
	root := &cobra.Command{
		Use:   "meshctl",
		Short: "meshctl — менеджер WireGuard mesh-сетей (multihop exit-маршруты)",
		Long: `meshctl управляет одноранговой mesh-сетью из WireGuard-нод
(Linux / Mikrotik / OpenWRT): YAML-конфиги в Git, маршруты через
несколько хопов, один бинарник без центрального сервера.

См. Plan-001.md для архитектуры и roadmap.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", config.DefaultConfigFile,
		"путь к главному YAML-конфигу")

	root.AddCommand(
		initCmd(),
		nodeCmd(),
		routeCmd(),
		planCmd(),
		statusCmd(),
		applyCmd(),
		clientConfigCmd(),
		tuiCmd(),
		versionCmd(),
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
			cmd.Printf("meshctl %s\n", Version)
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
