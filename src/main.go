package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
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
	path           string
	ff             *FormattedFile
	lineMap        []int
	abrs           []abridgement
	lineNoColWidth int
}

func (f *FileRegion) Height() int {
	return len(f.lineMap) + 1
}

func (f *FileRegion) Update(m *Model, msg tea.KeyMsg, cursor int) tea.Cmd {
	if msg.String() == "enter" {
		lineIdx := f.lineMap[cursor-1]
		if lineIdx < 0 {
			abrIdx := (-lineIdx) - 1
			f.abrs = append(f.abrs[:abrIdx], f.abrs[abrIdx+1:]...)
			f.updateLineMap()
		}
	}

	return nil
}

func (f *FileRegion) View(startLine int, numLines int, cursor int, m *Model) string {
	view := make([]string, numLines)

	view[0] = gloss.NewStyle().
		Width(m.w).
		Background(gloss.Color("#b9c902")).
		Foreground(gloss.Color("#000")).
		Render(fmt.Sprintf(" ▼ %s", f.path))

	for i := 1; i < numLines; i++ {
		lineIdx := f.lineMap[startLine+i-1]
		isCursor := i+startLine == cursor

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
			f.lineNoColWidth, line.aNum,
			f.lineNoColWidth, line.bNum,
			line.Render(background),
		)
	} else if line.mode == ADDED {
		lineContent = fmt.Sprintf(
			"%*s %*d +%s",
			f.lineNoColWidth, "",
			f.lineNoColWidth, line.bNum,
			line.Render(background),
		)
	} else {
		lineContent = fmt.Sprintf(
			"%*d %*s -%s",
			f.lineNoColWidth, line.aNum,
			f.lineNoColWidth, "",
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
		if abrIdx < len(f.abrs) && idx == f.abrs[abrIdx].start {
			f.lineMap = append(f.lineMap, -(abrIdx+1))
			idx = f.abrs[abrIdx].end + 1
			abrIdx++
		} else {
			f.lineMap = append(f.lineMap, idx)
			idx++
		}
	}
}

func newFileRegion(ff *FormattedFile, path string) *FileRegion {
	region := FileRegion{
		ff: ff,
		path: path,
	}

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

	jankLog(fmt.Sprintf("len(ff.lines): %d\n", len(ff.lines)))

	region.lineNoColWidth = GetLineNoColWidth(ff)
	region.updateLineMap()
	return &region
}

type Model struct {
	cursor         int
	w              int
	h              int
	x              int
	y              int
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
			region, relCursor := m.getCursorTarget()
			cmd := region.Update(&m, msg, relCursor)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) View() string {
	var parts []string
	cumY := 0

	for _, region := range m.regions {
		rH := region.Height()

		if cumY > m.y + m.h {
			// Got enough lines to paint a screen
			break
		}

		if cumY + rH < m.y {
			// Region is out of viewport
			continue
		}

		startLine := Max(m.y - cumY - rH, 0)
		linesToRender := Min(rH - startLine, m.y + m.h - cumY)
		cursor := m.cursor - cumY
		if m.cursor > cumY + rH || m.cursor < cumY {
			cursor = -1
		}

		parts = append(parts, region.View(startLine, linesToRender, cursor, &m))
		cumY += rH
	}

	return strings.Join(parts, "\n")
}

func (m Model) getCursorTarget() (VRegion, int) {
	cumY := 0

	for _, region := range m.regions {
		rH := region.Height()
		jankLog(fmt.Sprintf("rH: %d cumY: %d cursor: %d\n", rH, cumY, m.cursor))

		if m.cursor < cumY + rH && m.cursor >= cumY {
			return region, m.cursor - cumY
		}
		cumY += rH
	}
	panic("Unable to find the region the curor is currently in")
}

func (m Model) totalHeight() int {
	h := 0
	for _, region := range m.regions {
		h += region.Height()
	}

	return h
}

type CreateFileRegionMsg struct {
	idx  int
	pid  int
	path string
	diff string
	ref  string
}

func NewModel() Model {
	gl := GLInstance{apiUrl: "https://gitlab.bstock.io/api"}
	mrData, err := gl.FetchMR(400, 643)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	regions := make([]VRegion, len(mrData.Changes))
	q := make(chan CreateFileRegionMsg, 8)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			for msg := range q {
				baseContent, err := gl.FetchFileContents(msg.pid, msg.path, msg.ref)
				if err != nil {
					panic(err)
				}

				ff, err := FormatFile(*baseContent, msg.diff, "javascript")
				if err != nil {
					panic(err)
				}

				jankLog(fmt.Sprintf("ZZZ\nr: %s\nbaseContent: %s\nZZZ", msg.ref, *baseContent))
				regions[msg.idx] = newFileRegion(ff, msg.path)
			}
			wg.Done()
		}()

	}

	for idx, change := range mrData.Changes {
		q <- CreateFileRegionMsg{
			idx: idx,
			pid: 400,
			path: change.OldPath,
			diff: change.Diff,
			ref: mrData.TargetBranch,
		}
	}
	close(q)
	wg.Wait()

	model := Model{
		regions: regions,
		w: 80,
		h: 24,
	}

	return model
}

func main() {
	jankLog("\n\n====== NEW RUN ======\n\n")
	model := NewModel()
	p := tea.NewProgram(model)

	model.Init()
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
