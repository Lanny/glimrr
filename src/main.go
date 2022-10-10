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

const NUM_FR_TYPES = 4

const (
	FRLine   int = 0
	FRHeader     = 1
	FRAbr        = 2
	FRComment    = 3
)

type FileRegion struct {
	ff             *FormattedFile

	oldPath        string
	newPath        string
	added          bool
	removed        bool

	collapsed      bool

	lineMap        []int
	abrs           []abridgement
	lineNoColWidth int
}

type VRegion interface {
	Height() int
	Update(m *Model, msg tea.KeyMsg, cursor int) tea.Cmd
	View(startLine int, numLines int, cursor int, m *Model) string
	FullyExpand()
}

type abridgement struct {
	start int
	end   int
}

func (f *FileRegion) Height() int {
	if f.collapsed {
		return 1
	} else {
		return len(f.lineMap)
	}
}

func (f *FileRegion) Update(m *Model, msg tea.KeyMsg, cursor int) tea.Cmd {
	objIdx, objType := DivMod(f.lineMap[cursor], NUM_FR_TYPES)

	if msg.String() == "enter" {
		if objType == FRAbr {
			f.abrs = append(f.abrs[:objIdx], f.abrs[objIdx+1:]...)
			f.updateLineMap()
		} else if objType == FRHeader {
			f.collapsed = !f.collapsed
		}
	}

	return nil
}

func (f *FileRegion) View(startLine int, numLines int, cursor int, m *Model) string {
	if numLines < 1 {
		return ""
	}

	view := make([]string, numLines)

	ecSymbol := "▼"
	if f.collapsed {
		ecSymbol = "▶"
	}

	modeString := ""
	if f.added {
		modeString = " [NEW]"
	} else if f.removed {
		modeString = " [DELETED]"
	}

	// Render the file header
	view[0] = gloss.NewStyle().
		Width(m.w).
		Background(gloss.Color("#b9c902")).
		Foreground(gloss.Color("#000")).
		Render(fmt.Sprintf(" %s %s%s", ecSymbol, f.newPath, modeString))

	// Start from 1 to ignore space for header 
	for i := 1; i < numLines; i++ {
		objIdx, objType := DivMod(f.lineMap[startLine+i], NUM_FR_TYPES)
		isCursor := i+startLine == cursor

		if objType == FRLine{
			line := f.ff.lines[objIdx]
			view[i] = f.renderLine(line, isCursor, m)
		} else if objType == FRAbr {
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
		} else {
			view[i] = gloss.NewStyle().
				Width(m.w).
				Background(gloss.Color(bgColorMap[0])).
				Render("Whoops!")
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
			"%*d %*d   %s",
			f.lineNoColWidth, line.aNum,
			f.lineNoColWidth, line.bNum,
			line.Render(background),
		)
	} else if line.mode == ADDED {
		lineContent = fmt.Sprintf(
			"%*s %*d + %s",
			f.lineNoColWidth, "",
			f.lineNoColWidth, line.bNum,
			line.Render(background),
		)
	} else {
		lineContent = fmt.Sprintf(
			"%*d %*s - %s",
			f.lineNoColWidth, line.aNum,
			f.lineNoColWidth, "",
			line.Render(background),
		)
	}

	return gloss.NewStyle().
		Width(m.w).
		Background(background).
		Inline(true).
		MaxWidth(m.w).
		Render(lineContent)
}

func (f *FileRegion) FullyExpand() {
	f.abrs = f.abrs[:0]
	f.updateLineMap()
}


func (f *FileRegion) updateLineMap() {
	f.lineMap = make([]int, 1)
	lineIdx := 0
	abrIdx := 0

	f.lineMap[0] = FRHeader

	for lineIdx < len(f.ff.lines) {
		if abrIdx < len(f.abrs) && lineIdx == f.abrs[abrIdx].start {
			f.lineMap = append(f.lineMap, (abrIdx * NUM_FR_TYPES) + FRAbr)
			lineIdx = f.abrs[abrIdx].end + 1
			abrIdx++
		} else {
			f.lineMap = append(f.lineMap, (lineIdx * NUM_FR_TYPES) + FRLine)
			lineIdx++
		}
	}
}

func newFileRegion(ff *FormattedFile, change GLChangeData) *FileRegion {
	region := FileRegion{
		ff: ff,
		oldPath: change.OldPath,
		newPath: change.NewPath,
		added: change.NewFile,
		removed: change.DeletedFile,
		collapsed: change.DeletedFile,
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

		case "e":
			for _, region := range m.regions {
				region.FullyExpand()
			}
		case "ctrl+d":
			totalHeight := m.totalHeight()
			m.y = Min(m.y+(m.h+1)/2, totalHeight-1-m.h)
			m.cursor = Min(m.cursor+(m.h+1)/2, totalHeight-1)
		case "ctrl+u":
			m.y = Max(m.y-m.h/2, 0)
			m.cursor = Max(m.cursor-m.h/2, 0)
		default:
			region, relCursor := m.getCursorTarget()
			cmd := region.Update(&m, msg, relCursor)
			return m, cmd
		}
	}
	jankLog(fmt.Sprintf("c: %d, y: %d\n", m.cursor, m.y))

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
			cumY += rH
			continue
		}

		startLine := Max(m.y - cumY, 0)
		linesToRender := Min(Min(rH - startLine, m.y + m.h - cumY), m.h)
		cursor := m.cursor - cumY
		if m.cursor > cumY + rH || m.cursor < cumY {
			cursor = -1
		}

		parts = append(parts, region.View(startLine, linesToRender, cursor, &m))
		cumY += rH
	}

	if len(parts) > m.h {
		ln("Warning: Viewport height it %d but view is %d lines high", m.h, len(parts))
	}

	background := gloss.Color(bgColorMap[0])
	return gloss.NewStyle().
		Width(m.w).
		Height(m.h).
		MaxWidth(m.w).
		MaxHeight(m.h).
		Background(background).
		Render(strings.Join(parts, "\n"))
}

func (m Model) getCursorTarget() (VRegion, int) {
	cumY := 0

	for _, region := range m.regions {
		rH := region.Height()

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
	idx    int
	pid    int
	ref    string
	change GLChangeData
}

func NewModel() Model {
	pid := 39953668
	mrid := 1

	gl := GLInstance{apiUrl: "https://gitlab.com/api"}
	gl.Init()
	mrData, err := gl.FetchMR(pid, mrid)
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
				var baseContent string

				if !msg.change.NewFile {
					fetchedContent, err := gl.FetchFileContents(
						msg.pid,
						msg.change.OldPath,
						msg.ref,
					)
					if err != nil {
						panic(err)
					}
					baseContent = *fetchedContent
				} else {
					baseContent = ""
				}

				ff, err := FormatFile(baseContent, msg.change)
				if err != nil {
					panic(err)
				}

				regions[msg.idx] = newFileRegion(ff, msg.change)
			}
			wg.Done()
		}()

	}

	for idx, change := range mrData.Changes {
		q <- CreateFileRegionMsg{
			idx: idx,
			pid: pid,
			change: change,
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
