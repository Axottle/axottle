package installui

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestUnicodeEditingAndArrowSequences(t *testing.T) {
	m := newModel(nil, &bytes.Buffer{}, func() {}, "", "ru")
	m.mode = "input"
	m.edit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("привет.example")})
	m.edit(tea.KeyMsg{Type: tea.KeyHome})
	m.edit(tea.KeyMsg{Type: tea.KeyRight})
	m.edit(tea.KeyMsg{Type: tea.KeyDelete})
	m.edit(tea.KeyMsg{Type: tea.KeyBackspace})
	m.edit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("При")})
	if got := string(m.input); got != "Приивет.example" {
		t.Fatal(got)
	}
	m.edit(tea.KeyMsg{Type: tea.KeyCtrlU})
	if got := string(m.input); got != "ивет.example" {
		t.Fatal(got)
	}
	m.edit(tea.KeyMsg{Type: tea.KeyEnd})
	m.edit(tea.KeyMsg{Type: tea.KeyCtrlW})
	if len(m.input) != 0 || m.cursor != 0 {
		t.Fatal(string(m.input), m.cursor)
	}
}

func TestPasteCannotSubmitAnotherAnswer(t *testing.T) {
	var answers bytes.Buffer
	m := newModel(nil, &answers, func() {}, "", "en")
	m.mode = "input"
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("one\n\rtwo\x00"), Paste: true})
	if answers.Len() != 0 || string(m.input) != "onetwo" {
		t.Fatal(answers.String(), string(m.input))
	}
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	command()
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(answers.String()))
	if err != nil || string(decoded) != "onetwo" {
		t.Fatal(string(decoded), err)
	}
}

func TestSecretAndLongInputStayInsideField(t *testing.T) {
	m := newModel(nil, &bytes.Buffer{}, func() {}, "", "ru")
	m.input = []rune(strings.Repeat("секрет", 40))
	m.cursor, m.secret = len(m.input), true
	view := m.inputView(24)
	if ansi.StringWidth(view) > 24 || strings.Contains(view, "секрет") {
		t.Fatal(view)
	}
	m.cursor = 2
	if ansi.StringWidth(m.inputView(24)) > 24 {
		t.Fatal(m.inputView(24))
	}
}

func TestTokenHelpRemainsVisibleWhileEditing(t *testing.T) {
	m := newModel(nil, &bytes.Buffer{}, func() {}, "", "en")
	m.Update(event{kind: "input", fields: []string{"Cloudflare API token", "", "Zone DNS Edit and Zone Read", "https://dash.cloudflare.com/profile/api-tokens?name=Axottle", "secret"}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sensitive-token")})
	view := m.View()
	for _, expected := range []string{"Cloudflare API token", "Zone DNS Edit", "https://dash.cloudflare.com/profile/api-tokens"} {
		if !strings.Contains(ansi.Strip(view), expected) {
			t.Fatalf("Missing %q in %s", expected, view)
		}
	}
	if strings.Contains(view, "sensitive-token") {
		t.Fatal("token displayed")
	}
}

func TestModelDoesNotSwitchTerminalScreens(t *testing.T) {
	m := newModel(nil, &bytes.Buffer{}, func() {}, "", "en")
	for _, e := range []event{
		{kind: "notice", fields: []string{"Welcome", "Continue"}},
		{kind: "step", fields: []string{"Download"}},
		{kind: "status", fields: []string{"Verifying signatures"}},
		{kind: "input", fields: []string{"Domain", "", "", "", ""}},
		{kind: "result", fields: []string{"https://panel.example.com/setup"}},
	} {
		m.Update(e)
		if strings.Contains(m.View(), "1049") {
			t.Fatal("model changed terminal screen")
		}
	}
}

func TestProtocolRejectsMalformedRequests(t *testing.T) {
	for _, input := range []string{"noise", "AXUI1\tmenu", "AXUI1\tinput\t%%%", "AXUI1\tunknown\t"} {
		if _, err := parseEvent(input); err == nil {
			t.Fatal(input)
		}
	}
}

func TestWindowBoundsWithLongStatusAndSmallTerminal(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {48, 20}, {120, 40}} {
		m := newModel(nil, &bytes.Buffer{}, func() {}, "", "ru")
		m.width, m.height = size[0], size[1]
		for range 6 {
			m.Update(event{kind: "status", fields: []string{strings.Repeat("Проверка длинного имени сервера ", 4)}})
		}
		lines := strings.Split(m.View(), "\n")
		if len(lines) > m.height {
			t.Fatalf("height %d > %d", len(lines), m.height)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > m.width {
				t.Fatalf("width %d > %d", ansi.StringWidth(line), m.width)
			}
		}
	}
}

func TestCompletionContainsBothPanelsAndRevealsPasswordOnlyOnRequest(t *testing.T) {
	m := newModel(nil, &bytes.Buffer{}, func() {}, "", "ru")
	m.width, m.height = 100, 40
	m.Update(event{kind: "result", fields: []string{"https://panel.example/setup", "false", "yes", "https://remna.example", "admin", "generated-fixture-password", "/etc/axottle/axottle.env"}})
	m.Update(exited{})
	view := ansi.Strip(m.View())
	for _, expected := range []string{"01  Axottle", "02  Remnawave", "https://remna.example", "admin", "/etc/axottle/axottle.env", "P — показать пароль"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("Missing %q in %s", expected, view)
		}
	}
	if strings.Contains(view, "generated-fixture-password") {
		t.Fatal("password visible before request")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !strings.Contains(m.View(), "generated-fixture-password") {
		t.Fatal("password not revealed")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("з")})
	if strings.Contains(m.View(), "generated-fixture-password") {
		t.Fatal("password not hidden with Russian keyboard")
	}
}

func TestCompletionScrollsAfterBackendExitAndFitsSmallWindows(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {48, 20}, {120, 40}} {
		m := newModel(nil, &bytes.Buffer{}, func() {}, "", "en")
		m.width, m.height = size[0], size[1]
		m.Update(event{kind: "result", fields: []string{"https://panel.example/setup", "false", "partial", "https://remna.example", "admin", strings.Repeat("x", 100), "/etc/axottle/axottle.env"}})
		m.Update(exited{})
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		initial := m.View()
		m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		if m.scroll != 1 {
			t.Fatal("completed screen cannot scroll")
		}
		if size[1] == 20 && initial == m.View() {
			t.Fatal("scroll did not reveal clipped content")
		}
		lines := strings.Split(m.View(), "\n")
		if len(lines) > m.height {
			t.Fatal("completion exceeds height")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > m.width {
				t.Fatal("completion exceeds width")
			}
		}
	}
}

func TestPreviewAndPanelOnlyCompletionDoNotShowRemnawaveCredentials(t *testing.T) {
	for _, fields := range [][]string{
		{"https://panel.example/setup", "false"},
		{"https://panel.example/setup", "true", "yes", "https://remna.example", "admin", "fixture-password", "/etc/axottle/axottle.env"},
	} {
		m := newModel(nil, &bytes.Buffer{}, func() {}, "", "en")
		m.Update(event{kind: "result", fields: fields})
		m.Update(exited{})
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		if view := m.View(); strings.Contains(view, "fixture-password") || strings.Contains(view, "02  Remnawave") {
			t.Fatal("unexpected credentials")
		}
	}
}
