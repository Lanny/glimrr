package main

import (
	"fmt"
	"os"
	"strings"
	//  "github.com/alecthomas/chroma"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
)

var bgColorMap = [...]string{
	"#000",
	"#040",
	"#400",
	"#000",
	"#444",
	"#474",
	"#744",
}

func jankLog(msg string) {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	f.WriteString(msg)
	defer f.Close()
}

type VRegion interface {
	Height() int
	Update(m *Model, msg tea.KeyMsg, cursor int) tea.Cmd
	View(startLine int, numLines int, cursor int, m *Model) string
}

type abridgement struct {
	start int
	end   int
}

type FileRegion struct {
	ff      *FormattedFile
	lineMap []int
	abrs    []abridgement
}

func (f *FileRegion) Height() int {
	return len(f.lineMap)
}

func (f *FileRegion) Update(m *Model, msg tea.KeyMsg, cursor int) tea.Cmd {
	if msg.String() == "enter" {
		lineIdx := f.lineMap[cursor]
		if lineIdx < 0 {
			abrIdx := (-lineIdx) - 1
			f.abrs = append(f.abrs[:abrIdx], f.abrs[abrIdx+1:]...)
			f.updateLineMap()
		}
	}

	jankLog(fmt.Sprintf("%+v\n", f.abrs))
	return nil
}

func (f *FileRegion) View(startLine int, numLines int, cursor int, m *Model) string {
	view := make([]string, numLines)

	for i := 0; i < numLines; i++ {
		lineIdx := f.lineMap[startLine+i]
		isCursor := i+startLine == m.cursor

		if lineIdx >= 0 {
			line := f.ff.lines[lineIdx]
			view[i] = f.renderLine(line, isCursor, m)
		} else {
			var bgColor gloss.Color
			if isCursor {
				bgColor = gloss.Color(bgColorMap[4])
			} else {
				bgColor = gloss.Color(bgColorMap[0])
			}

			view[i] = gloss.NewStyle().
				Width(m.w).
				Align(gloss.Center).
				Background(bgColor).
				Render("...")
		}
	}

	return strings.Join(view, "\n")
}

func (f *FileRegion) renderLine(line *FormattedLine, cursor bool, m *Model) string {
	var lineContent string
	bgIdx := line.mode
	if cursor {
		bgIdx = bgIdx | 4
	}
	background := gloss.Color(bgColorMap[bgIdx])

	if line.mode == UNCHANGED {
		lineContent = fmt.Sprintf(
			"%*d %*d  %s",
			m.lineNoColWidth, line.aNum,
			m.lineNoColWidth, line.bNum,
			line.Render(background),
		)
	} else if line.mode == ADDED {
		lineContent = fmt.Sprintf(
			"%*s %*d +%s",
			m.lineNoColWidth, "",
			m.lineNoColWidth, line.bNum,
			line.Render(background),
		)
	} else {
		lineContent = fmt.Sprintf(
			"%*d %*s -%s",
			m.lineNoColWidth, line.aNum,
			m.lineNoColWidth, "",
			line.Render(background),
		)
	}

	return gloss.NewStyle().
		Width(m.w).
		Background(background).
		Render(lineContent)
}

func (f *FileRegion) updateLineMap() {
	f.lineMap = make([]int, 0)
	idx := 0
	abrIdx := 0

	for idx < len(f.ff.lines) {
		if idx == f.abrs[abrIdx].start {
			f.lineMap = append(f.lineMap, -(abrIdx+1))
			idx = f.abrs[abrIdx].end + 1
			abrIdx++
		} else {
			f.lineMap = append(f.lineMap, idx)
			idx++
		}
	}
}

func newFileRegion(ff *FormattedFile) *FileRegion {
	region := FileRegion{ff: ff}

	inNonAbr := false
	lastNonAbrEnd := 0
	linesWithoutChange := 0

	for idx, line := range ff.lines {
		if line.mode == UNCHANGED {
			linesWithoutChange++

			if inNonAbr && linesWithoutChange >= 10 {
				inNonAbr = false
				lastNonAbrEnd = idx - 5
			}
		} else {
			linesWithoutChange = 0

			if !inNonAbr {
				inNonAbr = true
				region.abrs = append(region.abrs, abridgement{
					start: lastNonAbrEnd,
					end:   Max(0, Max(lastNonAbrEnd, idx-5)),
				})
			}
		}
	}

	if !inNonAbr {
		region.abrs = append(region.abrs, abridgement{
			start: lastNonAbrEnd,
			end:   len(ff.lines) - 1,
		})
	}

	region.updateLineMap()

	jankLog(fmt.Sprintf("%+v\n", region.abrs))

	return &region
}

type Model struct {
	cursor         int
	w              int
	h              int
	x              int
	y              int
	lineNoColWidth int
	regions        []VRegion
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--

				if m.cursor < m.y {
					m.y = m.cursor
				}
			}

		case "down", "j":
			totalHeight := m.totalHeight()
			if m.cursor < totalHeight-1 {
				m.cursor++

				if m.cursor >= m.y+m.h {
					m.y = m.cursor - m.h + 1
				}
			}

		case "ctrl+d":
			totalHeight := m.totalHeight()
			m.y = Min(m.y+m.h/2, totalHeight-1-m.h)
			m.cursor = Min(m.cursor+m.h/2, totalHeight-2)
		case "ctrl+u":
			m.y = Max(m.y-m.h/2, 0)
			m.cursor = Max(m.cursor-m.h/2, 0)
		default:
			cmd := m.regions[0].Update(&m, msg, m.cursor)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) View() string {
	jankLog(fmt.Sprintf("y: %d, h: %d, th: %d\n", m.y, m.h, m.totalHeight()))
	return m.regions[0].View(m.y, Min(m.h, m.totalHeight()), m.cursor, &m)
}

func (m Model) totalHeight() int {
	h := 0
	for _, region := range m.regions {
		h += region.Height()
	}

	return h
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

	ff, err := FormatFile(string(bcontents), string(dcontents), "javascript")
	if err != nil {
		panic(err)
	}

	//TestFormat(string(bcontents))

	model := Model{
		lineNoColWidth: GetLineNoColWidth(ff),
	}
	model.regions = append(model.regions, newFileRegion(ff))

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
