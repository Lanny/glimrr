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
  lineNoColWidth int
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


func (m Model) View() string {
  var lineContent string
  view := make([]string, m.h)

  for i := 0; i < m.h; i++ {
    sourceIdx := m.y + i
    line := m.diff.lines[sourceIdx]

    if line.mode == UNCHANGED {
      lineContent = fmt.Sprintf(
        "  %*d %*d %s",
        m.lineNoColWidth, line.aNum,
        m.lineNoColWidth, line.bNum,
        line.text,
      )
    } else if line.mode == ADDED {
      lineContent = fmt.Sprintf(
        "+ %*s %*d %s",
        m.lineNoColWidth, "",
        m.lineNoColWidth, line.bNum,
        line.text,
      )
    } else {
      lineContent = fmt.Sprintf(
        "- %*d %*s %s",
        m.lineNoColWidth, line.aNum,
        m.lineNoColWidth, "",
        line.text,
      )
    }

    if sourceIdx == m.cursor {
      view[i] = cursorStyle.Width(m.w).Render(lineContent)
    } else {
      view[i] = lineContent
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
    diff: df,
    lineNoColWidth: GetLineNoColWidth(df),
  }

  return model
}

func main() {
  model := NewModel()
  p := tea.NewProgram(model)

  model.Init()
  if err := p.Start(); err != nil {
    fmt.Printf("Alas, there's been an error: %v", err)
    os.Exit(1)
  }
}
