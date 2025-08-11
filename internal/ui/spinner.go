package ui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RunSpinner runs a minimal Bubble Tea spinner while executing the given action.
// The UI exits when the action completes and returns the action's error.
func RunSpinner(ctx context.Context, title string, action func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Capture stdout while spinner is running to avoid interleaving output.
	// We render the spinner to stderr so user-facing output (stdout) prints
	// cleanly after the spinner finishes.
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		// Fallback: if we cannot create a pipe, just run spinner normally
		m := newSpinnerModel(ctx, title, action)
		p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
		if _, err := p.Run(); err != nil {
			return err
		}
		return m.err
	}

	// Replace os.Stdout with pipe writer and set up a log channel for live spinner text updates
	os.Stdout = w
	logCh := make(chan logEntry, 100)
	setActiveLogChannel(logCh)

	var wg sync.WaitGroup
	var buf bytes.Buffer
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
	}()

	// Ensure restoration even on panic/early return
	defer func() {
		// Close writer to end reader goroutine
		_ = w.Close()
		// Clear spinner log channel
		clearActiveLogChannel()
		// Restore stdout
		os.Stdout = oldStdout
		// Wait for the reader to finish draining
		wg.Wait()
		// Close read end
		_ = r.Close()
		// Flush captured output after spinner is done
		if buf.Len() > 0 {
			_, _ = oldStdout.Write(buf.Bytes())
		}
	}()

	// Pass action; spinner model will read from the active log channel
	m := newSpinnerModel(ctx, title, action)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	if _, err := p.Run(); err != nil {
		return err
	}
	return m.err
}

type actionDoneMsg struct{ err error }

type spinnerModel struct {
	ctx   context.Context
	title string
	spin  spinner.Model
	done  bool
	err   error
	style lipgloss.Style
	// logging integration
	logCh       chan logEntry
	currentText string
}

func newSpinnerModel(ctx context.Context, title string, action func() error) *spinnerModel {
	if ctx == nil {
		ctx = context.Background()
	}
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Attach any active logger channel so we can update the text as logs arrive
	activeLogMu.RLock()
	ch := activeLogCh
	activeLogMu.RUnlock()

	m := &spinnerModel{
		ctx:   ctx,
		title: title,
		spin:  s,
		style: lipgloss.NewStyle().Padding(0, 1),
		logCh: ch,
	}

	// Kick off the action in the background and notify on completion
	go func() {
		// Small delay for smoother paint before heavy work
		time.Sleep(50 * time.Millisecond)
		err := action()
		m.err = err
		m.done = true
	}()

	return m
}

func (m *spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, waitForCompletion(m))
}

func waitForCompletion(m *spinnerModel) tea.Cmd {
	return func() tea.Msg {
		// Poll until the action goroutine marks as done or context is canceled
		for {
			select {
			case <-m.ctx.Done():
				return actionDoneMsg{err: m.ctx.Err()}
			default:
				if m.done {
					return actionDoneMsg{err: m.err}
				}
				time.Sleep(75 * time.Millisecond)
			}
		}
	}
}

func (m *spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Drain any pending logs non-blockingly and update current text
	for {
		if m.logCh == nil {
			break
		}
		select {
		case e := <-m.logCh:
			// Use the last message as the current text
			if e.message != "" {
				m.currentText = e.message
			}
		default:
			// no more logs
			goto afterDrain
		}
	}
afterDrain:
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			// Allow cancel via keyboard
			m.err = fmt.Errorf("operation canceled")
			return m, tea.Quit
		}
	case actionDoneMsg:
		m.err = msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, tea.Batch(cmd, waitForCompletion(m))
	}
	return m, nil
}

func (m *spinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return m.style.Render("✗ " + m.title + " (" + m.err.Error() + ")\n")
		}
		return m.style.Render("✓ " + m.title + "\n")
	}
	text := m.title
	if m.currentText != "" {
		text = m.currentText
	}
	return m.style.Render(m.spin.View() + " " + text)
}
