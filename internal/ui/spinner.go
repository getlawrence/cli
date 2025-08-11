package ui

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "os"
    "sync"
    "time"
)

// RunSpinner executes the provided action while showing a very simple text spinner.
// The spinner writes to stderr to avoid interfering with normal stdout output.
// Stdout is captured during the run and flushed afterwards to prevent interleaving.
func RunSpinner(ctx context.Context, title string, action func() error) error {
    if ctx == nil {
        ctx = context.Background()
    }

    // Try to capture stdout to avoid interleaving with the spinner on stderr.
    oldStdout := os.Stdout
    r, w, pipeErr := os.Pipe()
    if pipeErr != nil {
        // Fallback: run without stdout capture
        return runSimpleSpinner(ctx, title, action, nil)
    }

    // Redirect stdout
    os.Stdout = w

    var wg sync.WaitGroup
    var buf bytes.Buffer
    wg.Add(1)
    go func() {
        defer wg.Done()
        _, _ = io.Copy(&buf, r)
    }()

    // Ensure restoration even on panic/early return
    defer func() {
        _ = w.Close()
        os.Stdout = oldStdout
        wg.Wait()
        _ = r.Close()
        if buf.Len() > 0 {
            _, _ = oldStdout.Write(buf.Bytes())
        }
    }()

    // Provide a channel for live text updates from Logf
    logCh := make(chan logEntry, 100)
    setActiveLogChannel(logCh)
    defer clearActiveLogChannel()

    return runSimpleSpinner(ctx, title, action, logCh)
}

func runSimpleSpinner(ctx context.Context, title string, action func() error, logCh <-chan logEntry) error {
    frames := []string{"-", "\\", "|", "/"}
    frameIndex := 0

    var (
        err        error
        done       = make(chan struct{})
        currentMsg = title
        lineWidth  = 0
    )

    // Kick off the action
    go func() {
        // slight delay for smoother first paint
        time.Sleep(40 * time.Millisecond)
        err = action()
        close(done)
    }()

    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    // helper to render a line padded to clear previous content
    render := func(text string) {
        frame := frames[frameIndex%len(frames)]
        line := fmt.Sprintf("\r%s %s", frame, text)
        // pad with spaces if new line shorter than previous
        if len(line) < lineWidth {
            line = line + spaces(lineWidth-len(line))
        }
        lineWidth = len(line)
        _, _ = fmt.Fprint(os.Stderr, line)
    }

    // initial paint
    render(currentMsg)

    for {
        select {
        case <-ctx.Done():
            // show final canceled state
            render(title + " (canceled)")
            _, _ = fmt.Fprintln(os.Stderr)
            return ctx.Err()
        case <-done:
            if err != nil {
                render("✗ " + title + " (" + err.Error() + ")")
            } else {
                render("✓ " + title)
            }
            _, _ = fmt.Fprintln(os.Stderr)
            return err
        case <-ticker.C:
            // drain any pending log updates without blocking
            if logCh != nil {
                for {
                    select {
                    case e := <-logCh:
                        if e.message != "" {
                            currentMsg = e.message
                        }
                    default:
                        goto afterDrain
                    }
                }
            }
        afterDrain:
            frameIndex++
            render(currentMsg)
        }
    }
}

func spaces(n int) string {
    if n <= 0 {
        return ""
    }
    const blanks = "                                                                "
    // repeat in chunks of 64
    s := ""
    for n > 0 {
        if n >= len(blanks) {
            s += blanks
            n -= len(blanks)
        } else {
            s += blanks[:n]
            n = 0
        }
    }
    return s
}
