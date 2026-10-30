package installui

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type event struct {
	kind   string
	fields []string
}

type exited struct{ err error }

func parseEvent(line string) (event, error) {
	parts := strings.Split(line, "\t")
	if len(parts) < 2 || parts[0] != "AXUI1" {
		return event{}, errors.New("invalid installer UI message")
	}
	e := event{kind: parts[1]}
	for _, field := range parts[2:] {
		value, err := base64.StdEncoding.DecodeString(field)
		if err != nil {
			return event{}, fmt.Errorf("invalid installer UI field: %w", err)
		}
		e.fields = append(e.fields, string(value))
	}
	minimum, known := map[string]int{"step": 1, "status": 1, "input": 5, "menu": 4, "notice": 1, "review": 1, "result": 1, "error": 1, "language": 1}[e.kind]
	if !known || len(e.fields) < minimum {
		return event{}, errors.New("incomplete installer UI message")
	}
	return e, nil
}

// Run keeps the terminal under one renderer. The installer uses separate pipes
// for prompts and answers; subprocess stdout cannot overwrite the interface.
func Run(script string, args []string) error {
	// OSC background queries can consume the first keystroke in SSH clients
	// that do not answer them. Use the advertised palette without a query.
	background := os.Getenv("COLORFGBG")
	lipgloss.SetHasDarkBackground(!strings.HasSuffix(background, ";7") && !strings.HasSuffix(background, ";15"))
	profile := termenv.ANSI256
	if os.Getenv("COLORTERM") == "truecolor" || os.Getenv("COLORTERM") == "24bit" {
		profile = termenv.TrueColor
	}
	if os.Getenv("NO_COLOR") != "" {
		profile = termenv.Ascii
	}
	lipgloss.SetColorProfile(profile)
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open an interactive SSH terminal: %w", err)
	}
	defer tty.Close()
	eventsRead, eventsWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer eventsRead.Close()
	defer eventsWrite.Close()
	answersRead, answersWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer answersRead.Close()
	defer answersWrite.Close()
	log, err := os.CreateTemp("", "axottle-install-output-*.log")
	if err != nil {
		return err
	}
	defer log.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), "AXOTTLE_INSTALL_UI=pipe")
	cmd.Stdout, cmd.Stderr = log, log
	cmd.ExtraFiles = []*os.File{eventsWrite, answersRead}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	eventsWrite.Close()
	answersRead.Close()
	stream := make(chan tea.Msg, 64)
	go func() {
		scanner := bufio.NewScanner(eventsRead)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			e, err := parseEvent(scanner.Text())
			if err != nil {
				cancel()
				stream <- event{kind: "error", fields: []string{err.Error()}}
				break
			}
			stream <- e
		}
		if err := scanner.Err(); err != nil {
			cancel()
			stream <- event{kind: "error", fields: []string{err.Error()}}
		}
		err := cmd.Wait()
		stream <- exited{err}
		close(stream)
	}()
	m := newModel(stream, answersWrite, cancel, log.Name(), language(args))
	result, runErr := tea.NewProgram(m, tea.WithInput(tty), tea.WithOutput(tty), tea.WithAltScreen()).Run()
	if runErr != nil {
		return runErr
	}
	finished := result.(*model)
	if finished.err != nil {
		return fmt.Errorf("installation stopped: %w; log: %s", finished.err, log.Name())
	}
	if !finished.finished {
		return errors.New("installation interrupted")
	}
	return nil
}

func sendAnswer(writer io.Writer, answer string) tea.Cmd {
	return func() tea.Msg {
		_, err := fmt.Fprintln(writer, base64.StdEncoding.EncodeToString([]byte(answer)))
		if err != nil {
			return event{kind: "error", fields: []string{"Cannot send answer to installer: " + err.Error()}}
		}
		return nil
	}
}
