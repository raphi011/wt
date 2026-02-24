---
name: charmbracelet-tui
description: Use when building or modifying terminal UI with Bubbletea, Bubbles, or Lipgloss v2. Use when creating tea.Model implementations, handling keyboard/mouse input, managing program lifecycle, styling terminal output, using bubbles components (textinput, progress, spinner), or writing tests for TUI code.
---

# Charmbracelet TUI Development (v2)

Best practices for building terminal UIs with the Charmbracelet stack: **Bubbletea** (framework), **Bubbles** (components), **Lipgloss** (styling). All libraries are v2 with `charm.land` import paths.

## Import Paths

```go
tea "charm.land/bubbletea/v2"           // framework
"charm.land/bubbles/v2/textinput"       // text input component
"charm.land/bubbles/v2/progress"        // progress bar component
"charm.land/bubbles/v2/spinner"         // spinner component
"charm.land/lipgloss/v2"               // styling
"charm.land/lipgloss/v2/table"         // table component
"github.com/charmbracelet/colorprofile" // terminal color detection
```

## The Elm Architecture

Bubbletea follows the Elm Architecture. Every interactive component implements `tea.Model`:

```go
type Model interface {
    Init() tea.Cmd                          // initial command (e.g., start spinner)
    Update(tea.Msg) (tea.Model, tea.Cmd)    // handle messages, return new state + side effects
    View() tea.View                         // render current state (MUST be pure)
}
```

**Key principles:**
- **Update is the only place state changes** - View is a pure function of state
- **Side effects are tea.Cmd** (`func() tea.Msg`) - never perform I/O in Update/View
- **Messages drive everything** - keyboard input, window resize, custom events all arrive as `tea.Msg`

## View Returns tea.View (Not String)

```go
// Always wrap output in tea.NewView()
func (m model) View() tea.View {
    return tea.NewView("rendered content")
}

// Empty view (e.g., when done)
return tea.NewView("")
```

`tea.View` has declarative fields that replace v1 commands:

```go
v := tea.NewView(content)
v.AltScreen = true                          // replaces tea.EnterAltScreen
v.MouseMode = tea.MouseModeCellMotion       // replaces tea.EnableMouseCellMotion
v.ReportFocus = true                        // replaces tea.EnableReportFocus
v.WindowTitle = "My App"                    // replaces tea.SetWindowTitle
```

**Note:** `View.Content` is a `string` - compare with `""`, never `nil`.

## Keyboard Input

Use `tea.KeyPressMsg` (not the v1 `tea.KeyMsg`):

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "enter":    // enter key
    case "ctrl+c":   // ctrl combinations
    case "space":    // space bar (NOT " ")
    case "up":       // arrow keys
    case "esc":      // escape
    case "q":        // character keys
    }
```

**Field access** for programmatic matching:

```go
msg.Code    // rune: tea.KeyEnter, tea.KeyUp, 'a', ' ', etc.
msg.Text    // string: typed text (e.g., "a")
msg.Mod     // modifier: tea.ModCtrl, tea.ModAlt, tea.ModShift
msg.Key()   // full key info struct
```

**Common key constants:** `tea.KeyEnter`, `tea.KeyEscape`, `tea.KeyUp`, `tea.KeyDown`, `tea.KeyLeft`, `tea.KeyRight`, `tea.KeyHome`, `tea.KeyEnd`, `tea.KeyTab`, `tea.KeyBackspace`, `tea.KeyDelete`

## Mouse Input

Mouse messages are split by event type:

```go
case tea.MouseClickMsg:     // button pressed
case tea.MouseReleaseMsg:   // button released
case tea.MouseWheelMsg:     // scroll wheel
case tea.MouseMotionMsg:    // movement

// Access mouse data
mouse := msg.Mouse()
x, y := mouse.X, mouse.Y
```

## Program Creation and Lifecycle

```go
p := tea.NewProgram(model,
    tea.WithOutput(os.Stderr),          // ALWAYS for piping support
    tea.WithColorProfile(profile),      // explicit color profile
    tea.WithoutSignalHandler(),         // for background/embedded programs
)
finalModel, err := p.Run()
```

**Always output to stderr** when stdout needs to be pipeable (e.g., `cd $(wt cd -i)`):

```go
tea.WithOutput(os.Stderr)
```

**Color profile detection** (pair with WithColorProfile for correct rendering):

```go
profile := colorprofile.Detect(os.Stderr, os.Environ())
p := tea.NewProgram(model,
    tea.WithOutput(os.Stderr),
    tea.WithColorProfile(profile),
)
```

## Commands and Messages

Commands are side effects that produce messages:

```go
// A command is func() tea.Msg
func fetchData() tea.Msg {
    result, err := api.Get()
    if err != nil {
        return errMsg{err}
    }
    return dataMsg{result}
}

// Return from Update
return m, fetchData   // runs async, sends result back as message

// Built-in commands
tea.Quit              // quit the program
tea.Batch(cmd1, cmd2) // run commands in parallel
tea.Sequence(cmd1, cmd2) // run commands sequentially (v2: renamed from Sequentially)
```

### Channel-Based Messages (Background Updates)

For long-running operations that push updates:

```go
type progressUpdate struct {
    current int
    message string
}

func waitForUpdate(ch chan progressUpdate) tea.Cmd {
    return func() tea.Msg {
        msg, ok := <-ch
        if !ok {
            return tea.Quit()  // channel closed -> quit
        }
        return msg
    }
}

// In Update: re-subscribe after handling
case progressUpdate:
    m.current = msg.current
    return m, waitForUpdate(m.updateCh)
```

## Bubbles Components

### TextInput

```go
ti := textinput.New()
ti.Placeholder = "Enter value..."
ti.CharLimit = 156
ti.Prompt = "> "
ti.SetWidth(40)

// Cursor styling
styles := ti.Styles()
styles.Cursor.Shape = tea.CursorBar     // also: tea.CursorBlock, tea.CursorUnderline
styles.Cursor.Blink = true
styles.Focused.Text = myStyle           // style when focused
styles.Blurred.Text = myStyle           // style when blurred
ti.SetStyles(styles)

// Focus management
ti.Focus()                              // activate input
ti.Blur()                               // deactivate
ti.Focused()                            // check state

// Init must return Blink for cursor animation
func (m model) Init() tea.Cmd {
    m.input.Focus()
    return textinput.Blink
}

// Forward messages in Update
m.input, cmd = m.input.Update(msg)
```

### Progress

```go
prog := progress.New(
    progress.WithWidth(40),
    progress.WithoutPercentage(),
    progress.WithColors(primaryColor, accentColor),  // variadic color.Color
)

bar := prog.ViewAs(0.75)  // render at 75%

// Forward Update for animations
prog, cmd = prog.Update(msg)
```

### Table (lipgloss/v2/table)

```go
t := table.New().
    Headers("NAME", "STATUS", "COUNT").
    Rows(rows...).
    BorderTop(false).
    BorderBottom(false).
    BorderLeft(false).
    BorderRight(false).
    BorderHeader(false).
    BorderColumn(false).
    BorderRow(false).
    StyleFunc(func(row, col int) lipgloss.Style {
        if row == table.HeaderRow {
            return lipgloss.NewStyle().Bold(true).PaddingRight(2)
        }
        return lipgloss.NewStyle().PaddingRight(2)
    })

output := t.String()
```

## Lipgloss Styling

### Style Creation

```go
style := lipgloss.NewStyle().
    Foreground(lipgloss.Color("62")).   // ANSI color
    Bold(true).
    Italic(true).
    Underline(true).
    Padding(0, 1).
    MarginTop(1)

rendered := style.Render("text")
```

### Colors

```go
lipgloss.Color("62")        // ANSI 256
lipgloss.Color("#ff0000")   // hex
lipgloss.NoColor{}          // terminal default (no color override)
```

`lipgloss.Color()` returns `color.Color` (`image/color`). Use this type for color variables:

```go
import "image/color"

var Primary color.Color = lipgloss.Color("62")
```

### Background Detection

```go
isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stderr)
```

### Character-Level Styling (Fuzzy Match Highlights)

```go
// Highlight specific character positions with one style, rest with another
lipgloss.StyleRunes(text, matchedIndices, highlightStyle, normalStyle)
```

### Borders

```go
lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).       // also: NormalBorder, ThickBorder, DoubleBorder
    BorderForeground(primaryColor).
    BorderLeft(true)                        // enable specific sides
```

## Style Architecture Pattern

Define a central theme with semantic color roles, then build styles as functions (not variables) to support runtime theme switching:

```go
// Theme struct with semantic colors (styles/theme.go)
type Theme struct {
    Primary color.Color   // borders, titles
    Accent  color.Color   // selected/active items
    Success color.Color   // checkmarks
    Error   color.Color   // error messages
    Muted   color.Color   // disabled text
}

// Styles as functions to pick up theme changes (framework/styles.go)
func TitleStyle() lipgloss.Style {
    return lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
}

func SelectedStyle() lipgloss.Style {
    return lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
}
```

**Why functions not variables:** Package-level `var` styles capture colors at init time. If the theme changes at runtime (e.g., from config), those variables are stale. Style functions read current color values on each call.

## Testing Patterns

### Synthetic Key Events

```go
func keyMsg(key string) tea.KeyPressMsg {
    switch key {
    case "enter":
        return tea.KeyPressMsg{Code: tea.KeyEnter}
    case "up":
        return tea.KeyPressMsg{Code: tea.KeyUp}
    case "down":
        return tea.KeyPressMsg{Code: tea.KeyDown}
    case "left":
        return tea.KeyPressMsg{Code: tea.KeyLeft}
    case "right":
        return tea.KeyPressMsg{Code: tea.KeyRight}
    case "esc":
        return tea.KeyPressMsg{Code: tea.KeyEscape}
    case "ctrl+c":
        return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
    default:
        if len(key) == 1 {
            return tea.KeyPressMsg{Code: rune(key[0]), Text: key}
        }
        return tea.KeyPressMsg{}
    }
}
```

### Testing Models Directly

Test by calling `Update()` with synthetic messages - no need to run a `tea.Program`:

```go
m := newModel()
m.Init()

// Send key events directly
updated, cmd := m.Update(keyMsg("enter"))
m = updated.(*myModel)

// Assert state
if !m.done { t.Error("expected done") }
```

### View Assertions

```go
view := m.View()
if view.Content == "" {     // Content is string in v2, NOT nil-comparable
    t.Error("expected non-empty view")
}
```

### Type-Safe Step Testing (Generic Helper)

For testing subcomponents that return their own type (not `tea.Model`):

```go
func updateStep[T framework.Step](t *testing.T, s T, msg tea.KeyPressMsg) (T, framework.StepResult) {
    t.Helper()
    newStep, _, result := s.Update(msg)
    return newStep.(T), result
}
```

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| `View() string` | `View() tea.View` + `tea.NewView()` |
| `case tea.KeyMsg:` | `case tea.KeyPressMsg:` |
| `case " ":` for space | `case "space":` |
| `tea.Sequentially()` | `tea.Sequence()` |
| `view.Content == nil` | `view.Content == ""` (string in v2) |
| `tea.WithAltScreen()` option | `view.AltScreen = true` (declarative) |
| `tea.EnterAltScreen` command | `view.AltScreen = true` (declarative) |
| Printing to stdout | `tea.WithOutput(os.Stderr)` for piping |
| Missing color profile | `colorprofile.Detect()` + `tea.WithColorProfile()` |
| Style variables for themed UI | Style functions that read current theme |
| `os.Getwd()` in commands | Use context-injected working directory |
