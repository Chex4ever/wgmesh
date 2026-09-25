package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load читает mesh-конфиг из YAML-файла.
func Load(path string) (*Mesh, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать %s: %w", path, err)
	}
	m := &Mesh{}
	if err := yaml.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("не удалось разобрать %s: %w", path, err)
	}
	if m.Version == 0 {
		m.Version = 1
	}
	return m, nil
}

// Save атомарно записывает mesh-конфиг в YAML-файл.
func Save(path string, m *Mesh) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("не удалось сериализовать конфиг: %w", err)
	}
	header := []byte("# Конфигурация mesh-сети (wgmesh). Версионируется в Git.\n")
	data = append(header, data...)

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wgmesh-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// конфиги содержат приватные ключи — ограничиваем права
	return os.Chmod(path, 0o600)
}

// Exists проверяет наличие конфигурационного файла.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DefaultMesh — дефолтный конфиг для `wgmesh init`.
func DefaultMesh() *Mesh {
	return &Mesh{
		Name:     "My Mesh Network",
		Version:  1,
		CIDR:     DefaultCIDR,
		Nodes:    []Node{},
		Routes:   []Route{},
	}
}

