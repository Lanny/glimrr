package main

import (
  "fmt"
  "os"
  "strings"
//  "github.com/alecthomas/chroma"
  gloss "github.com/charmbracelet/lipgloss"
  tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
  diff *DiffFile
  cursor int
  w int
  h int
  x int
  y int
}

func (m Model) Init() tea.Cmd {
  return nil
}

var cursorStyle = gloss.NewStyle().Background(gloss.Color("#AAAAAA"))

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  switch msg := msg.(type) {
  case tea.WindowSizeMsg:
    m.w = msg.Width
    m.h = msg.Height

  case tea.KeyMsg:
    switch msg.String() {
      case "ctrl+c", "q":
        return m, tea.Quit

      // The "up" and "k" keys move the cursor up
      case "up", "k":
        if m.cursor > 0 {
          m.cursor--

          if m.cursor < m.y {
            m.y = m.cursor
          }
        }

      // The "down" and "j" keys move the cursor down
      case "down", "j":
        if m.cursor < len(m.diff.lines)-1 {
          m.cursor++

          if m.cursor >= m.y + m.h {
            m.y = m.cursor - m.h + 1
          }
        }
    }
  }

  return m, nil
}

func Min(x, y int) int {
  if x > y {
    return y
  }
  return x
}

func (m Model) View() string {
  view := make([]string, m.h)

  for i := 0; i < m.h; i++ {
    sourceIdx := m.y + i

    if sourceIdx == m.cursor {
      view[i] = cursorStyle.Width(m.w).Render(m.diff.lines[sourceIdx].text)
    } else {
      view[i] = m.diff.lines[sourceIdx].text
    }
  }

  return strings.Join(view, "\n")
}

func NewModel() Model {
  dcontents, err := os.ReadFile("./test-data/taxDoc.diff")
  if err != nil {
    panic(err)
  }

  bcontents, err := os.ReadFile("./test-data/taxDoc.js")
  if err != nil {
    panic(err)
  }

  df, err := AnnotateWithDiff(string(bcontents), string(dcontents))
  if err != nil {
    panic(err)
  }

  model := Model{
    cursor: 0,
    x: 0,
    y: 0,
    w: 80,
    h: 24,
    diff: df,
  }

  return model
}

func main() {
  model := NewModel()
  p := tea.NewProgram(model)

  if err := p.Start(); err != nil {
    fmt.Printf("Alas, there's been an error: %v", err)
    os.Exit(1)
  }
}
