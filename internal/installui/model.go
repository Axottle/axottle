package installui

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type tick time.Time

type model struct {
	stream                         <-chan tea.Msg
	answers                        io.Writer
	cancel                         func()
	logPath, lang                  string
	width, height                  int
	mode, title, hint, link, stage string
	options                        []string
	selected                       int
	input                          []rune
	cursor                         int
	secret                         bool
	entries                        []string
	result                         string
	err                            error
	finished, stopping             bool
	preview                        bool
	started                        time.Time
	frame                          int
	scroll                         int
}

func language(args []string) string {
	for i := range args {
		if args[i] == "--lang" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if strings.HasPrefix(os.Getenv("LANG"), "ru") {
		return "ru"
	}
	return "en"
}

func newModel(stream <-chan tea.Msg, answers io.Writer, cancel func(), logPath, lang string) *model {
	return &model{stream: stream, answers: answers, cancel: cancel, logPath: logPath,
		lang: lang, width: 80, height: 24, mode: "progress", started: time.Now()}
}

func (m *model) tr(ru, en string) string {
	if m.lang == "ru" {
		return ru
	}
	return en
}
func (m *model) next() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.stream
		if !ok {
			return nil
		}
		return msg
	}
}
func pulse() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}
func (m *model) Init() tea.Cmd { return tea.Batch(m.next(), pulse()) }

func field(e event, i int) string {
	if i >= len(e.fields) {
		return ""
	}
	return e.fields[i]
}

// Control bytes from pasted input or subprocess messages are never rendered.
func plain(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tick:
		m.frame++
		return m, pulse()
	case exited:
		m.finished, m.err = true, msg.err
		if msg.err != nil {
			if m.mode != "error" {
				m.title = m.tr("Установка остановлена", "Installation stopped")
			}
			m.mode = "error"
			m.hint = m.tr("Подробности сохранены в журнале:", "Details were saved to the log:") + "\n" + m.logPath
		} else {
			m.mode, m.title = "done", m.tr("Всё готово", "You’re all set")
			m.hint = m.tr("Откройте панель и создайте администратора.", "Open the panel and create your administrator account.")
			if m.preview {
				m.title = m.tr("Предпросмотр завершён", "Preview complete")
				m.hint = m.tr("Система не изменялась. Адрес ниже — пример.", "No system changes. The address below is an example.")
			}
		}
		return m, nil
	case event:
		m.scroll = 0
		switch msg.kind {
		case "language":
			m.lang = field(msg, 0)
		case "step", "status":
			text := plain(field(msg, 0))
			if msg.kind == "step" {
				m.stage = text
			}
			m.entries = append(m.entries, text)
			if len(m.entries) > 6 {
				m.entries = m.entries[len(m.entries)-6:]
			}
			if m.mode != "error" {
				m.mode = "progress"
			}
		case "input":
			m.mode, m.title = "input", plain(field(msg, 0))
			m.input = []rune(strings.ReplaceAll(plain(field(msg, 1)), "\n", ""))
			m.cursor = len(m.input)
			m.hint, m.link, m.secret = plain(field(msg, 2)), field(msg, 3), field(msg, 4) == "secret"
		case "menu":
			m.mode, m.title, m.hint = "menu", plain(field(msg, 0)), plain(field(msg, 1))
			m.selected, _ = strconv.Atoi(field(msg, 2))
			m.options = nil
			for _, option := range msg.fields[3:] {
				m.options = append(m.options, plain(option))
			}
			if m.selected < 0 || m.selected >= len(m.options) {
				m.selected = 0
			}
		case "notice", "review":
			m.mode, m.title = msg.kind, plain(field(msg, 0))
			m.hint = plain(strings.Join(msg.fields[1:], "\n"))
		case "result":
			m.result = plain(field(msg, 0))
			m.preview = field(msg, 1) == "true"
		case "error":
			m.mode, m.title = "error", plain(field(msg, 0))
		}
		return m, m.next()
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlJ {
			msg.Type = tea.KeyEnter
		}
		if msg.Type == tea.KeyCtrlC {
			if m.finished {
				return m, tea.Quit
			}
			m.stopping = true
			m.cancel()
			return m, nil
		}
		if m.finished {
			if msg.Type == tea.KeyEnter || msg.Type == tea.KeyEsc {
				return m, tea.Quit
			}
			return m, nil
		}
		if msg.Type == tea.KeyPgDown {
			m.scroll++
			return m, nil
		}
		if msg.Type == tea.KeyPgUp {
			m.scroll = max(0, m.scroll-1)
			return m, nil
		}
		switch m.mode {
		case "input":
			if msg.Type == tea.KeyEnter {
				answer := string(m.input)
				m.input, m.cursor, m.mode = nil, 0, "progress"
				return m, sendAnswer(m.answers, answer)
			}
			m.edit(msg)
		case "menu":
			if len(m.options) == 0 {
				return m, nil
			}
			switch msg.String() {
			case "up", "k":
				m.selected = (m.selected + len(m.options) - 1) % len(m.options)
			case "down", "j", "tab":
				m.selected = (m.selected + 1) % len(m.options)
			case "enter":
				m.mode = "progress"
				return m, sendAnswer(m.answers, strconv.Itoa(m.selected))
			}
		case "notice", "review":
			if msg.Type == tea.KeyEnter {
				m.mode = "progress"
				return m, sendAnswer(m.answers, "")
			}
		}
	}
	return m, nil
}

func (m *model) edit(key tea.KeyMsg) {
	switch key.Type {
	case tea.KeyLeft, tea.KeyCtrlB:
		m.cursor = max(0, m.cursor-1)
	case tea.KeyRight, tea.KeyCtrlF:
		m.cursor = min(len(m.input), m.cursor+1)
	case tea.KeyHome, tea.KeyCtrlA:
		m.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.cursor = len(m.input)
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.cursor > 0 {
			m.input = append(m.input[:m.cursor-1], m.input[m.cursor:]...)
			m.cursor--
		}
	case tea.KeyDelete, tea.KeyCtrlD:
		if m.cursor < len(m.input) {
			m.input = append(m.input[:m.cursor], m.input[m.cursor+1:]...)
		}
	case tea.KeyCtrlU:
		m.input = m.input[m.cursor:]
		m.cursor = 0
	case tea.KeyCtrlK:
		m.input = m.input[:m.cursor]
	case tea.KeyCtrlW:
		start := m.cursor
		for start > 0 && unicode.IsSpace(m.input[start-1]) {
			start--
		}
		for start > 0 && !unicode.IsSpace(m.input[start-1]) {
			start--
		}
		m.input = append(m.input[:start], m.input[m.cursor:]...)
		m.cursor = start
	case tea.KeyRunes, tea.KeySpace:
		chars := key.Runes
		if key.Type == tea.KeySpace {
			chars = []rune{' '}
		}
		clean := make([]rune, 0, len(chars))
		for _, r := range chars {
			if !unicode.IsControl(r) {
				clean = append(clean, r)
			}
		}
		if len(m.input)+len(clean) > 8192 {
			return
		}
		value := append([]rune(nil), m.input[:m.cursor]...)
		value = append(value, clean...)
		value = append(value, m.input[m.cursor:]...)
		m.input, m.cursor = value, m.cursor+len(clean)
	}
}

var accent = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#963F4B", Dark: "#FC9CA1"})
var muted = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#62585C", Dark: "#B6A7AD"})

func (m *model) inputView(width int) string {
	value := append([]rune(nil), m.input...)
	if m.secret {
		for i := range value {
			value[i] = '•'
		}
	}
	start := 0
	for start < m.cursor && ansi.StringWidth(string(value[start:m.cursor])) >= width-1 {
		start++
	}
	left := string(value[start:m.cursor])
	caret, right := " ", ""
	if m.cursor < len(value) {
		caret = string(value[m.cursor])
		right = string(value[m.cursor+1:])
	}
	available := max(0, width-ansi.StringWidth(left)-ansi.StringWidth(caret))
	return left + accent.Reverse(true).Render(caret) + ansi.Truncate(right, available, "")
}

func linkView(raw string, width int) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || strings.ContainsAny(raw, "\x1b\r\n\a") {
		return plain(raw)
	}
	label := u.Scheme + "://" + u.Host + u.Path
	return ansi.SetHyperlink(raw) + accent.Underline(true).Render(ansi.Truncate(label, width, "…")) + ansi.ResetHyperlink()
}

func (m *model) View() string {
	if m.width < 18 || m.height < 8 {
		return m.tr("Увеличьте окно терминала. Ctrl+C — остановить.", "Enlarge the terminal. Ctrl+C stops installation.")
	}
	width := max(10, min(86, m.width-4)-6)
	height := max(1, min(27, m.height-2)-8)
	title, hint := m.title, m.hint
	footer := m.tr("Enter — продолжить   Ctrl+C — остановить", "Enter continue   Ctrl+C stop")
	var body []string
	switch m.mode {
	case "progress":
		title = m.tr("Устанавливаем Axottle", "Installing Axottle")
		if m.stage == "" {
			m.stage = m.tr("Подготовка", "Preparing setup")
		}
		hint = fmt.Sprintf("%s  %s · %s", []string{"◐", "◓", "◑", "◒"}[m.frame%4], m.stage, time.Since(m.started).Round(time.Second))
		for _, entry := range m.entries {
			body = append(body, muted.Render(ansi.Wrap("  "+entry, width, "")))
		}
		footer = m.tr("Ctrl+C — остановить", "Ctrl+C stop")
	case "menu":
		limit := max(1, height-3)
		start := max(0, m.selected-limit+1)
		for i := start; i < min(len(m.options), start+limit); i++ {
			line := "  " + ansi.Truncate(m.options[i], width-3, "…")
			if i == m.selected {
				line = accent.Bold(true).Render("› " + ansi.Truncate(m.options[i], width-3, "…"))
			}
			body = append(body, line)
		}
		footer = m.tr("↑↓ — выбрать   Enter — продолжить   Ctrl+C — выйти", "↑↓ select   Enter continue   Ctrl+C stop")
	case "input":
		if m.link != "" {
			body = append(body, linkView(m.link, width), "")
		}
		body = append(body, accent.Render("› ")+m.inputView(width-3))
		footer = m.tr("←→ Home End — курсор   Enter — продолжить", "←→ Home End edit   Enter continue   Ctrl+C stop")
	case "done":
		body = append(body, linkView(m.result, width))
		footer = m.tr("Enter — закрыть", "Enter close")
	case "error":
		footer = m.tr("Enter — закрыть", "Enter close")
	case "review":
		footer = m.tr("Enter — установить   Ctrl+C — отменить", "Enter install   Ctrl+C cancel")
	}
	if m.stopping && !m.finished {
		title, hint = m.tr("Останавливаем установку", "Stopping installation"), m.tr("Ждём завершения текущих процессов.", "Waiting for running processes to stop.")
	}
	content := lipgloss.NewStyle().Bold(true).Render(ansi.Wrap(plain(title), width, ""))
	if hint != "" {
		content += "\n\n" + muted.Render(ansi.Wrap(plain(hint), width, ""))
	}
	if len(body) > 0 {
		content += "\n\n" + strings.Join(body, "\n")
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		start := min(m.scroll, len(lines)-height)
		// Keep the editable field visible while retaining help through PgUp/PgDn.
		if m.mode == "input" && m.scroll == 0 {
			start = len(lines) - height
		}
		lines = lines[start : start+height]
		footer += m.tr(" · PgUp/PgDn — текст", " · PgUp/PgDn scroll")
	}
	content = strings.Join(lines, "\n")
	content = lipgloss.NewStyle().Width(width).Height(height).Render(content)
	header := accent.Bold(true).Render("Axottle") + muted.Render(m.tr("  /  Установка", "  /  Installation"))
	card := lipgloss.NewStyle().Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#CAB8BE", Dark: "#63515A"}).
		Render(header + "\n\n" + content + "\n\n" + muted.Render(ansi.Truncate(footer, width, "…")))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}
