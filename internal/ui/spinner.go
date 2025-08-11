package ui

import (
	"context"
	"fmt"
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
	m := newSpinnerModel(ctx, title, action)
	p := tea.NewProgram(m)
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
}

func newSpinnerModel(ctx context.Context, title string, action func() error) *spinnerModel {
	if ctx == nil {
		ctx = context.Background()
	}
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := &spinnerModel{
		ctx:   ctx,
		title: title,
		spin:  s,
		style: lipgloss.NewStyle().Padding(0, 1),
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
	return m.style.Render(m.spin.View() + " " + m.title)
}
