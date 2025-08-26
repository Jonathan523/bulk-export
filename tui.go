package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	stepText = iota
	stepScholars
	stepPath
	stepConfirm
)

type tuiModel struct {
	step         int
	text         textinput.Model
	path         textinput.Model
	options      []string
	keys         []string
	cursor       int
	selected     map[int]bool
	selectedKeys []string
	err          error
	done         bool
}

func initialModel() tuiModel {
	ti := textinput.New()
	ti.Placeholder = "输入要查询的文本"
	ti.Focus()
	pi := textinput.New()
	pi.Placeholder = "output.csv"
	return tuiModel{step: stepText, text: ti, path: pi, selected: make(map[int]bool)}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepText:
			switch msg.String() {
			case "enter":
				value := strings.TrimSpace(m.text.Value())
				if value == "" {
					m.err = fmt.Errorf("文本不能为空")
					return m, nil
				}
				for _, r := range value {
					if !unicode.Is(unicode.Han, r) {
						m.err = fmt.Errorf("包含非法字符 %q", r)
						return m, nil
					}
				}
				rows, err := fetchRows(string([]rune(value)[0]))
				if err != nil {
					m.err = err
					return m, nil
				}
				// build scholar list
				seen := make(map[string]struct{})
				for _, r := range rows {
					key := r.Era + "|" + r.Nature + "|" + r.Scholar
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					m.keys = append(m.keys, key)
					m.options = append(m.options, r.Scholar+"("+r.Era+" "+r.Nature+")")
				}
				m.step = stepScholars
				m.err = nil
				return m, nil
			default:
				var cmd tea.Cmd
				m.text, cmd = m.text.Update(msg)
				return m, cmd
			}
		case stepScholars:
			switch msg.String() {
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.options)-1 {
					m.cursor++
				}
			case " ":
				if m.selected[m.cursor] {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = true
				}
			case "enter":
				for i := range m.options {
					if m.selected[i] {
						m.selectedKeys = append(m.selectedKeys, m.keys[i])
					}
				}
				m.path.SetValue("output.csv")
				m.path.Focus()
				m.step = stepPath
			}
			return m, nil
		case stepPath:
			switch msg.String() {
			case "enter":
				if v := strings.TrimSpace(m.path.Value()); v != "" {
					m.path.SetValue(v)
				} else {
					m.path.SetValue("output.csv")
				}
				m.step = stepConfirm
			default:
				var cmd tea.Cmd
				m.path, cmd = m.path.Update(msg)
				return m, cmd
			}
		case stepConfirm:
			if msg.String() == "enter" {
				m.done = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m tuiModel) View() string {
	switch m.step {
	case stepText:
		s := "请输入要查询的字符串并按回车确认:\n" + m.text.View()
		if m.err != nil {
			s += "\n" + m.err.Error()
		}
		return s
	case stepScholars:
		s := "选择需要保存的学者（空格选择，回车确认）：\n\n"
		for i, opt := range m.options {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}
			check := " "
			if m.selected[i] {
				check = "x"
			}
			s += fmt.Sprintf("%s[%s] %s\n", cursor, check, opt)
		}
		return s
	case stepPath:
		return "输入保存路径并回车：\n" + m.path.View()
	case stepConfirm:
		return fmt.Sprintf("文本: %s\n选中学者: %d\n输出: %s\n\n回车开始导出", m.text.Value(), len(m.selectedKeys), m.path.Value())
	}
	return ""
}

func runTUI() (string, []string, string, error) {
	m := initialModel()
	p := tea.NewProgram(m)
	res, err := p.Run()
	if err != nil {
		return "", nil, "", err
	}
	fm := res.(tuiModel)
	if !fm.done {
		return "", nil, "", fmt.Errorf("aborted")
	}
	return fm.text.Value(), fm.selectedKeys, fm.path.Value(), nil
}
