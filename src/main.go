package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
	"regexp"
	"strconv"
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

type EndLoadingMsg struct{}
type ClearStatusMessageMsg struct {
	msgId int
}
type LoadMRMsg struct {
	regions []VRegion
	mr      GLMRData
	gl      *GLInstance
}

type ViewParams struct {
	x              int
	width          int
	lineNoColWidth int
}

type VRegion interface {
	Height() int
	Update(m *Model, msg tea.Msg, cursor int) (tea.Model, tea.Cmd)
	Resize(m *Model)
	View(startLine int, numLines int, cursor int, m *Model) string
	GetNextCursorTarget(lineNo int, direction int) int
	SetECState(value bool)
	GetPendingComments() []Comment
}

type StatusMessage struct {
	id  int
	msg string
}

type ModelInitData struct {
	glHost  string
	project string
	mrid    int

}

type Model struct {
	cursor      int
	w           int
	h           int
	x           int
	y           int
	mode        int
	loadingText string
	initData    ModelInitData
	gl          *GLInstance
	mr          GLMRData
	spinner     spinner.Model
	exInput     textinput.Model
	regions     []VRegion
	messages    []StatusMessage
	p           *tea.Program
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height
		m.exInput.Width = msg.Width
	case EndLoadingMsg:
		m.loadingText = ""
	case ClearStatusMessageMsg:
		removeIndex := -1
		for idx, message := range m.messages {
			if message.id == msg.msgId {
				removeIndex = idx
				break
			}
		}
		if removeIndex >= 0 {
			m.messages = append(m.messages[:removeIndex], m.messages[removeIndex+1:]...)
		}
		return m, nil
	case LoadMRMsg:
		m.loadingText = ""
		m.regions = msg.regions
		m.mr = msg.mr
		m.gl = msg.gl
		for _, region := range m.regions {
			region.Resize(&m)
		}
	}

	if m.loadingText != "" {
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	} else if m.mode == NormalMode {
		return m.nUpdate(msg)
	} else if m.mode == ExMode {
		return m.eUpdate(msg)
	} else {
		return m, nil
	}
}

func (m Model) nUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			(&m).moveCursor((m.h + 1) / 2)
		case "ctrl+u":
			m.y = Max(m.y-m.h/2, 0)
			(&m).moveCursor(-(m.h + 1) / 2)
		case ":":
			m.exInput = textinput.New()
			m.exInput.Focus()
			m.exInput.Prompt = ":"
			m.exInput.Width = m.w

			m.mode = ExMode
		default:
			region, relCursor := m.getCursorTarget(m.cursor)
			return region.Update(&m, msg, relCursor)
		}
	default:
		region, relCursor := m.getCursorTarget(m.cursor)
		return region.Update(&m, msg, relCursor)
	}

	return m, nil
}

func (m Model) eUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.exInput.SetValue("")
			m.mode = NormalMode
		case "enter":
			eCmd := m.exInput.Value()
			m.exInput.SetValue("")
			m.mode = NormalMode

			if eCmd == "q" || eCmd == "quit" {
				return m, tea.Quit
			}

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

			if eCmd == "Load" {
				return m.doBlockingLoad("Loading stuff...", func() tea.Msg {
					time.Sleep(3 * time.Second)
					return nil
				})
			}
			if eCmd == "Submit" {
				return m.doBlockingLoad("Submitting review...", func() tea.Msg {
					for _, region := range m.regions {
						for _, comment := range region.GetPendingComments() {
							note := comment.(*GLNote)
							m.gl.CreateComment(*note, m.mr)
						}
					}
					m.gl.InvalidateCache()

					return nil
				})
			}

			return m.displayStatusMessage(
				"ERR: Unrecognized command.",
				3*time.Second,
			)
		}
	}

	m.exInput, cmd = m.exInput.Update(msg)
	return m, cmd
}

func (m Model) doBlockingLoad(loadingMsg string, f tea.Cmd) (tea.Model, tea.Cmd) {
	m.spinner.Spinner = spinner.Dot
	m.loadingText = loadingMsg

	return m, tea.Batch(
		m.spinner.Tick,
		tea.Sequence(
			f,
			func() tea.Msg { return EndLoadingMsg{} },
		),
	)
}

func (m Model) displayStatusMessage(body string, clearAfter time.Duration) (tea.Model, tea.Cmd) {
	msg := StatusMessage{
		id:  rand.Intn(65535),
		msg: body,
	}

	if len(m.messages) > 0 {
		msg.id = m.messages[len(m.messages)-1].id + 1
	}
	m.messages = append(m.messages, msg)

	return m, tea.Tick(clearAfter, func(_ time.Time) tea.Msg {
		return ClearStatusMessageMsg{msgId: msg.id}
	})
}

func (m *Model) moveCursor(delta int) {
	totalHeight := m.totalHeight()
	prospective := Clamp(0, m.cursor+delta, totalHeight-1)
	region, relCursor := m.getCursorTarget(prospective)
	relTarget := region.GetNextCursorTarget(relCursor, delta)
	pTDelta := relTarget - relCursor

	m.cursor = prospective + pTDelta
}

func (m Model) View() string {
	log.Trace().
		Int("width", m.w).
		Int("height", m.h).
		Msg("Rendering...")
	background := CFG.Colors.Background

	if m.loadingText != "" {
		return gloss.NewStyle().
			Width(m.w).
			Height(m.h).
			Padding((m.h-1)/2, 0).
			Align(gloss.Center).
			Background(background).
			Render(fmt.Sprintf("%s %s", m.spinner.View(), m.loadingText))
	}

	var parts []string
	// target height for normal region rendering (ex mode input is the exception)
	tH := m.h
	// Height of accumulated rendering, so we know when to should stop
	cumY := 0

	if m.mode == ExMode {
		tH -= 1
	}

	tH -= len(m.messages)

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

	msgStyle := gloss.NewStyle().
		MaxWidth(m.w).
		MaxHeight(1)

	for _, msg := range m.messages {
		parts = append(parts, msgStyle.Render(msg.msg))
	}

	if m.mode == ExMode {
		parts = append(parts, m.exInput.View())
	}

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

func (m Model) Init() tea.Cmd {
	m.spinner.Spinner = spinner.Dot
	m.loadingText = ""
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			gl := GLInstance{apiUrl: fmt.Sprintf("%s/api", m.initData.glHost)}
			gl.Init()

			mrData, err := gl.FetchMR(m.initData.project, m.initData.mrid)
			if err != nil {
				panic(err)
			}

			regions := make([]VRegion, len(mrData.Changes))

			// Partion notes by file that they apply to
			notesByFile := make(map[string]([]Comment))
			for _, discussion := range mrData.Discussions {
				for _, note := range discussion.Notes {
					note := note
					if note.Type == "DiffNote" {
						path := note.Position.NewPath
						notesByFile[path] = append(notesByFile[path], &note)
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

						var comments []Comment
						var ok bool
						if comments, ok = notesByFile[msg.change.NewPath]; !ok {
							comments = nil
						}

						regions[msg.idx] = newFileRegion(ff, msg.change, comments, m.w)
					}
					wg.Done()
				}()

			}

			for idx, change := range mrData.Changes {
				q <- CreateFileRegionMsg{
					idx:    idx,
					pid:    m.initData.project,
					change: change,
					ref:    mrData.DiffRefs.BaseSHA,
				}
			}
			close(q)
			wg.Wait()

			return LoadMRMsg{
				regions: regions,
				mr:      *mrData,
				gl:      &gl,
			}
		},
	)
}

type CreateFileRegionMsg struct {
	idx    int
	pid    string
	ref    string
	change GLChangeData
}

//var logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

func main() {
	logLevels := map[string]zerolog.Level{
		"PANIC": zerolog.PanicLevel,
		"FATAL": zerolog.FatalLevel,
		"ERROR": zerolog.ErrorLevel,
		"WARN":  zerolog.WarnLevel,
		"INFO":  zerolog.InfoLevel,
		"DEBUG": zerolog.DebugLevel,
		"TRACE": zerolog.TraceLevel,
	}

	if logLevel, ok := logLevels[os.Getenv("LOG_LEVEL")]; ok {
		zerolog.SetGlobalLevel(logLevel)
		logfile, _ := os.OpenFile("glimrr.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: logfile})
	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}


	log.Debug().Msg("Glimmr starting...")

	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := fmt.Sprintf("%s/.config/glimrr/config.json", homeDir)
		log.Debug().Msg(fmt.Sprintf("Attempting to read config from %s", configPath))
		userConfig, err := loadConfigFromFile(configPath)
		CFG = userConfig
		if err != nil {
			log.Info().Msg("Unable to load user config, using defaults.")
		}
	}

	model := Model{
		loadingText: "Loading MR...",
		h:           24,
		w:           80,
	}
	model.spinner.Spinner = spinner.Dot

	mrUrlRegex := regexp.MustCompile(`(?P<host>https?://[^/?#]+)/(?P<project>.*)/-/merge_requests/(?P<mrid>[0-9]+)`)

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "glimrr must be invoked with at least one argument.")
		os.Exit(2)
	}

	matches := mrUrlRegex.FindStringSubmatch(os.Args[1])

	log.Debug().Msg(fmt.Sprintf("Arg parse result: %+v", matches))
	if len(matches) < 4 {
		fmt.Fprintln(os.Stderr, "unable to parse url.")
		os.Exit(2)
	}

	// Should not be possible to fail.
	mrid, _ := strconv.Atoi(matches[3])
	model.initData = ModelInitData{
		glHost: matches[1],
		project: matches[2],
		mrid: mrid,
	}


	// This doesn't feel great, but we need to call program methods from the
	// model so *shrug*
	mp := &model
	program := tea.NewProgram(mp)
	mp.p = program

	log.Debug().Msg("Handing control of console over to tea.")
	if err := program.Start(); err != nil {
		log.Fatal().Err(err).Msg("")
		os.Exit(1)
	}
}
