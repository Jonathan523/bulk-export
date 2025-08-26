package main

import (
	"fmt"
	"unicode"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type tuiResult struct {
	text      string
	keys      []string
	output    string
	firstRows []Row
}

type item struct {
	key      string
	name     string
	selected bool
}

func (i item) Title() string {
	prefix := "[ ]"
	if i.selected {
		prefix = "[x]"
	}
	return prefix + " " + i.name
}

func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.name }

type model struct {
	step      int
	textInput textinput.Model
	pathInput textinput.Model
	list      list.Model
	items     []item
	text      string
	keys      []string
	output    string
	err       string
	firstRows []Row
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "輸入漢字"
	ti.Focus()
	return model{step: 0, textInput: ti}
}

func runTUI() (tuiResult, error) {
	m := initialModel()
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return tuiResult{}, err
	}
	fm := final.(model)
	return tuiResult{text: fm.text, keys: fm.keys, output: fm.output, firstRows: fm.firstRows}, nil
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case 0:
			switch msg.String() {
			case "enter":
				text := m.textInput.Value()
				if text == "" {
					m.err = "請輸入內容"
					return m, nil
				}
				for _, r := range text {
					if !unicode.Is(unicode.Han, r) {
						m.err = "僅支持漢字"
						return m, nil
					}
				}
				rows, err := fetchRows(string([]rune(text)[0]))
				if err != nil {
					m.err = err.Error()
					return m, nil
				}
				m.text = text
				m.firstRows = rows
				items := make([]list.Item, len(rows))
				m.items = make([]item, len(rows))
				for i, r := range rows {
					key := r.Era + "|" + r.Nature + "|" + r.Scholar
					name := r.Era + " " + r.Nature + " " + r.Scholar
					it := item{key: key, name: name, selected: true}
					items[i] = it
					m.items[i] = it
				}
				lst := list.New(items, list.NewDefaultDelegate(), 0, 0)
				lst.Title = "選擇學者(空格切換, 回車確認)"
				m.list = lst
				m.step = 1
				return m, nil
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		case 1:
			switch msg.String() {
			case " ":
				if sel, ok := m.list.SelectedItem().(item); ok {
					sel.selected = !sel.selected
					m.items[m.list.Index()] = sel
					m.list.SetItem(m.list.Index(), sel)
				}
				return m, nil
			case "enter":
				var keys []string
				for _, it := range m.items {
					if it.selected {
						keys = append(keys, it.key)
					}
				}
				if len(keys) == 0 {
					m.err = "至少選擇一位學者"
					return m, nil
				}
				m.keys = keys
				pi := textinput.New()
				pi.Placeholder = "output.csv"
				pi.SetValue("output.csv")
				pi.Focus()
				m.pathInput = pi
				m.step = 2
				return m, nil
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		case 2:
			switch msg.String() {
			case "enter":
				out := m.pathInput.Value()
				if out == "" {
					out = "output.csv"
				}
				m.output = out
				m.step = 3
				return m, nil
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.pathInput, cmd = m.pathInput.Update(msg)
			return m, cmd
		case 3:
			switch msg.String() {
			case "enter":
				return m, tea.Quit
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.step {
	case 0:
		return fmt.Sprintf("輸入文本:\n%s\n%s", m.textInput.View(), m.err)
	case 1:
		return m.list.View()
	case 2:
		return fmt.Sprintf("輸入保存路徑:\n%s\n", m.pathInput.View())
	case 3:
		return fmt.Sprintf("文本: %s\n學者: %d人\n保存到: %s\n回車開始, Ctrl+C退出", m.text, len(m.keys), m.output)
	}
	return ""
}
