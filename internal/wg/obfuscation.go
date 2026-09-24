package wg

import (
	"fmt"
	"strings"
)

// ObfuscationParams описывает параметры AmneziaWG (заголовки H1-H4, размеры S1-S4, джиттер Jc/Jmin/Jmax).
type ObfuscationParams struct {
	Preset string `yaml:"preset,omitempty"`
	H1     uint32 `yaml:"h1,omitempty"`
	H2     uint32 `yaml:"h2,omitempty"`
	H3     uint32 `yaml:"h3,omitempty"`
	H4     uint32 `yaml:"h4,omitempty"`
	S1     uint16 `yaml:"s1,omitempty"`
	S2     uint16 `yaml:"s2,omitempty"`
	S3     uint16 `yaml:"s3,omitempty"`
	S4     uint16 `yaml:"s4,omitempty"`
	Jc     uint16 `yaml:"jc,omitempty"`
	Jmin   uint16 `yaml:"jmin,omitempty"`
	Jmax   uint16 `yaml:"jmax,omitempty"`
}

// PresetParams возвращает эталонные параметры AmneziaWG по имени пресета.
func PresetParams(name string) (*ObfuscationParams, error) {
	switch strings.ToLower(name) {
	case "ampere":
		return &ObfuscationParams{Preset: "ampere", H1: 1, H2: 2, H3: 3, H4: 4, S1: 15, S2: 20, Jc: 4, Jmin: 10, Jmax: 50}, nil
	case "kwanyee":
		return &ObfuscationParams{Preset: "kwanyee", H1: 10, H2: 20, H3: 30, H4: 40, S1: 30, S2: 40, Jc: 8, Jmin: 20, Jmax: 100}, nil
	case "mirai":
		return &ObfuscationParams{Preset: "mirai", H1: 100, H2: 200, H3: 300, H4: 400, S1: 50, S2: 60, Jc: 12, Jmin: 30, Jmax: 150}, nil
	case "rise":
		return &ObfuscationParams{Preset: "rise", H1: 500, H2: 600, H3: 700, H4: 800, S1: 70, S2: 80, Jc: 16, Jmin: 40, Jmax: 200}, nil
	case "rsal":
		return &ObfuscationParams{Preset: "rsal", H1: 1000, H2: 2000, H3: 3000, H4: 4000, S1: 90, S2: 100, Jc: 20, Jmin: 50, Jmax: 250}, nil
	case "voyager":
		return &ObfuscationParams{Preset: "voyager", H1: 12345, H2: 23456, H3: 34567, H4: 45678, S1: 100, S2: 120, Jc: 25, Jmin: 60, Jmax: 300}, nil
	default:
		return nil, fmt.Errorf("неизвестный пресет AmneziaWG: %q", name)
	}
}

// RenderAmneziaBlock генерирует текстовый блок параметров обфускации для wg-quick / AmneziaWG.
func (p *ObfuscationParams) RenderAmneziaBlock() string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# AmneziaWG Obfuscation\n")
	if p.H1 > 0 {
		sb.WriteString(fmt.Sprintf("H1 = %d\n", p.H1))
	}
	if p.H2 > 0 {
		sb.WriteString(fmt.Sprintf("H2 = %d\n", p.H2))
	}
	if p.H3 > 0 {
		sb.WriteString(fmt.Sprintf("H3 = %d\n", p.H3))
	}
	if p.H4 > 0 {
		sb.WriteString(fmt.Sprintf("H4 = %d\n", p.H4))
	}
	if p.S1 > 0 {
		sb.WriteString(fmt.Sprintf("S1 = %d\n", p.S1))
	}
	if p.S2 > 0 {
		sb.WriteString(fmt.Sprintf("S2 = %d\n", p.S2))
	}
	if p.Jc > 0 {
		sb.WriteString(fmt.Sprintf("Jc = %d\n", p.Jc))
	}
	if p.Jmin > 0 {
		sb.WriteString(fmt.Sprintf("Jmin = %d\n", p.Jmin))
	}
	if p.Jmax > 0 {
		sb.WriteString(fmt.Sprintf("Jmax = %d\n", p.Jmax))
	}
	return sb.String()
}
