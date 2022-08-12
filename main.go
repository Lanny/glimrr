package main

import (
  "fmt"
  "os"
  "strings"
  gloss "github.com/charmbracelet/lipgloss"
  tea "github.com/charmbracelet/bubbletea"
)

type model struct {
  lines []string
  cursor int
  w int
  h int
  x int
  y int
}

func (m model) Init() tea.Cmd {
  return nil
}

var cursorStyle = gloss.NewStyle().Background(gloss.Color("#AAAAAA"))

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
        if m.cursor < len(m.lines)-1 {
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

func (m model) View() string {
  view := make([]string, m.h)

  for i := 0; i < m.h; i++ {
    sourceIdx := m.y + i

    if sourceIdx == m.cursor {
      view[i] = cursorStyle.Width(m.w).Render(m.lines[sourceIdx])
    } else {
      view[i] = m.lines[sourceIdx]
    }
  }

  return strings.Join(view, "\n")
}

func main() {
  contents, err := os.ReadFile("/Users/ryan.jenkins/buyer-portal/package.json")
  if err != nil {
    panic(err)
  }

  lines := strings.Split(string(contents), "\n")

  my_model := model{
    lines: lines,
    cursor: 60,
    x: 0,
    y: 2,
    w: 80,
    h: 24,
  }

  p := tea.NewProgram(my_model)
  if err := p.Start(); err != nil {
    fmt.Printf("Alas, there's been an error: %v", err)
    os.Exit(1)
  }
}
