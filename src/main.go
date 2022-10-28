package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/textinput"
	"os"
	"strings"
	"sync"
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


const (
	NormalMode int = 0
	ExMode         = 1
)


type ViewParams struct {
	x              int
	width          int
	lineNoColWidth int
}

func (n *GLNote) Height(vp *ViewParams) int {
	return gloss.Height(n.Render(vp, false))
}

func (n *GLNote) Render(vp *ViewParams, cursor bool) string {
	margin := vp.lineNoColWidth * 2 + 2
	bg := "#444"
	if cursor {
		bg = "#666"
	}

	block := gloss.NewStyle().
		Background(gloss.Color(bg)).
		Width(vp.width - margin - 1).
		MarginLeft(margin).
		Padding(0, 2).
		Border(gloss.NormalBorder(), false, false, false, true).
		BorderForeground(gloss.Color("#FFF")).
		BorderBackground(gloss.Color(bg)).
		Render(n.Author.Name + ":\n" + n.Body)

	return block
}

type VRegion interface {
	Height() int
	Update(m *Model, msg tea.KeyMsg, cursor int) tea.Cmd
	View(startLine int, numLines int, cursor int, m *Model) string
	GetNextCursorTarget(lineNo int, direction int) int
	SetECState(value bool)
}

type abridgement struct {
	start int
	end   int
}


type Model struct {
	cursor  int
	w       int
	h       int
	x       int
	y       int
	mode    int
	exInput textinput.Model
	regions []VRegion
	p       *tea.Program
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == NormalMode {
		return m.nUpdate(msg)
	} else if m.mode == ExMode {
		return m.eUpdate(msg)
	} else {
		return m, nil
	}
}

func (m Model) nUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height
		m.exInput.Width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			(&m).moveCursor(-1)

			if m.cursor < m.y {
				m.y = m.cursor
			}

		case "down", "j":
			(&m).moveCursor(1)
			if m.cursor >= m.y+m.h {
				m.y = m.cursor - m.h + 1
			}

		case "G":
			totalHeight := m.totalHeight()
			m.y = totalHeight - m.h
			m.cursor = totalHeight - 1
		case "ctrl+d":
			totalHeight := m.totalHeight()
			m.y = Min(m.y+(m.h+1)/2, totalHeight-m.h)
			(&m).moveCursor((m.h+1)/2)
		case "ctrl+u":
			m.y = Max(m.y-m.h/2, 0)
			(&m).moveCursor(-(m.h+1)/2)
		case ":":
			m.mode = ExMode
		default:
			region, relCursor := m.getCursorTarget(m.cursor)
			cmd := region.Update(&m, msg, relCursor)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) eUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height
		m.exInput.Width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.exInput.SetValue("")
			m.mode = NormalMode
		case "enter":
			eCmd := m.exInput.Value()
			if eCmd == "CollapseAll" {
				for _, region := range m.regions {
					region.SetECState(true)
				}
			}

			if eCmd == "ExpandAll" {
				for _, region := range m.regions {
					region.SetECState(false)
				}
			}
			if eCmd == "q" || eCmd == "quit" {
				return m, tea.Quit
			}

			m.exInput.SetValue("")
			m.mode = NormalMode
		}
	}

	m.exInput, cmd = m.exInput.Update(msg)
	return m, cmd
}

func (m *Model) moveCursor(delta int) {
	totalHeight := m.totalHeight()
	prospective := Clamp(0, m.cursor + delta, totalHeight - 1)
	region, relCursor := m.getCursorTarget(prospective)
	relTarget := region.GetNextCursorTarget(relCursor, delta)
	pTDelta := relTarget - relCursor

	m.cursor = prospective + pTDelta
}

func (m Model) View() string {
	var parts []string
	// target height for normal region rendering (ex mode input is the exception)
	tH := m.h
	// Height of accumulated rendering, so we know when to should stop
	cumY := 0

	if m.mode == ExMode {
		tH -= 1
	}

	for _, region := range m.regions {
		rH := region.Height()

		if cumY > m.y+tH {
			// Got enough lines to paint a screen
			break
		}

		if cumY+rH < m.y {
			// Region is out of viewport
			cumY += rH
			continue
		}

		startLine := Max(m.y-cumY, 0)
		linesToRender := Min(Min(rH-startLine, m.y+tH-cumY), tH)
		cursor := m.cursor - cumY
		if m.cursor > cumY+rH || m.cursor < cumY {
			cursor = -1
		}

		parts = append(parts, region.View(startLine, linesToRender, cursor, &m))
		cumY += rH
	}

	if m.mode == ExMode {
		parts = append(parts, m.exInput.View())
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

func (m Model) getCursorTarget(cursor int) (VRegion, int) {
	cumY := 0

	for _, region := range m.regions {
		rH := region.Height()

		if cursor < cumY+rH && cursor >= cumY {
			return region, cursor - cumY
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

	exi := textinput.New()
	exi.Focus()
	exi.Prompt = ":"
	exi.Width = 80

	model := Model{
		regions: make([]VRegion, len(mrData.Changes)),
		w:       80,
		h:       24,
		exInput: exi,
	}

	// Partion notes by file that they apply to
	notesByFile := make(map[string]([]GLNote))
	for _, discussion := range mrData.Discussions {
		for _, note := range discussion.Notes {
			if note.Type == "DiffNote" {
				path := note.Position.NewPath
				notesByFile[path] = append(notesByFile[path], note)
			}
		}
	}

	var wg sync.WaitGroup
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

				var notes []GLNote
				var ok bool
				if notes, ok = notesByFile[msg.change.NewPath]; !ok {
					notes = nil
				}

				model.regions[msg.idx] = newFileRegion(ff, msg.change, notes, model.w)
			}
			wg.Done()
		}()

	}

	for idx, change := range mrData.Changes {
		q <- CreateFileRegionMsg{
			idx:    idx,
			pid:    pid,
			change: change,
			ref:    mrData.DiffRefs.BaseSHA,
		}
	}
	close(q)
	wg.Wait()

	return model

}

func main() {
	jankLog("\n\n====== NEW RUN ======\n\n")
	model := NewModel()
	mp := &model
	program := tea.NewProgram(mp)
	mp.p = program

	model.Init()

	if err := program.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
