package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Tab       key.Binding
	Enter     key.Binding
	AddHop    key.Binding
	RemoveHop key.Binding
	Apply     key.Binding
	Save      key.Binding
	Quit      key.Binding
	Help      key.Binding
}

var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "вверх"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "вниз"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "влево/сдвиг"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "вправо/сдвиг"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "смена панели"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "выбрать/изменить"),
	),
	AddHop: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+", "добавить хоп"),
	),
	RemoveHop: key.NewBinding(
		key.WithKeys("-", "_"),
		key.WithHelp("-", "удалить хоп"),
	),
	Apply: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "применить"),
	),
	Save: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "сохранить"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "выход"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "справка"),
	),
}
