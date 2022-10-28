package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"os"
	"os/exec"
	"strings"
)

const NUM_FR_TYPES = 5

const (
	FRLine    int = 0
	FRHeader      = 1
	FRAbr         = 2
	FRComment     = 3
	FRBlank       = 4
)

type FileRegion struct {
	ff *FormattedFile

	oldPath string
	newPath string
	added   bool
	removed bool

	collapsed bool

	lineMap        []int
	abrs           []abridgement
	notes          []GLNote
	lineNoColWidth int
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
	vp := &ViewParams{
		x:              0,
		width:          m.w,
		lineNoColWidth: f.lineNoColWidth,
	}

	switch msg.String() {
	case "enter":
		if objType == FRAbr {
			f.abrs = append(f.abrs[:objIdx], f.abrs[objIdx+1:]...)
			f.updateLineMap(vp)
		} else if objType == FRHeader {
			f.collapsed = !f.collapsed
		}

	case "t":
		f.collapsed = !f.collapsed
	case "c":
		if objType != FRLine {
			return nil
		}

		tmpFile, err := os.CreateTemp("", "new-comment-*.md")
		if err != nil {
			panic("Unable to open file for creating a new comment.")
		}

		fname := tmpFile.Name()
		defer os.Remove(fname)

		m.p.ReleaseTerminal()

		cmd := exec.Command("/usr/bin/vi", fname)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Run()

		commentBody, err := os.ReadFile(fname)

		if err != nil {
			ln("Unable to read comment temp file!")
			return nil
		}
		ln("Successfully collected comment:\n%s", commentBody)

		line := f.ff.lines[objIdx]
		var oldLineNo int
		var newLineNo int

		if line.mode != ADDED {
			oldLineNo = line.aNum
		}

		if line.mode != REMOVED {
			newLineNo = line.bNum
		}

		draftNote := GLNote{
			Id:   -1,
			Type: "DiffNote",
			Body: string(commentBody),
			Author: GLAuthor{
				Id:       -1,
				Name:     "(you)",
				Username: "(you)",
			},
			Position: GLPosition{
				PositionType: "text",
				OldPath:      f.oldPath,
				NewPath:      f.newPath,
				OldLine:      oldLineNo,
				NewLine:      newLineNo,
			},
		}
		f.notes = append(f.notes, draftNote)
		f.updateLineMap(vp)

		m.p.RestoreTerminal()
		return nil
	}

	return nil
}

func (f *FileRegion) View(startLine int, numLines int, cursor int, m *Model) string {
	vp := &ViewParams{
		x:              0,
		width:          m.w,
		lineNoColWidth: f.lineNoColWidth,
	}

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

		if objType == FRLine {
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
		} else if objType == FRComment {
			note := f.notes[objIdx]
			blockLines := strings.Split(note.Render(vp, isCursor), "\n")
			for _, line := range blockLines {
				view[i] = line
				i++
			}
			i--
		} else if objType == FRBlank {
			view[i] = gloss.NewStyle().
				Width(m.w).
				Background(gloss.Color(bgColorMap[0])).
				Render(".")
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

func (f *FileRegion) GetNextCursorTarget(lineNo int, direction int) int {
	i := lineNo
	d := Signum(direction)

	for {
		if i >= len(f.lineMap) || i < 0 {
			d = -d
			i += d
			continue
		}

		_, objType := DivMod(f.lineMap[i], NUM_FR_TYPES)
		if objType != FRBlank {
			break
		}

		i += d
	}

	return i
}

func (f *FileRegion) SetECState(value bool) {
	f.collapsed = value
}

func (f *FileRegion) updateLineMap(vp *ViewParams) {
	f.lineMap = make([]int, 1)
	lineIdx := 0
	abrIdx := 0
	noteIndex := make(map[string]([]int))

	for nidx, note := range f.notes {
		var key string
		if note.Position.NewLine == 0 {
			key = fmt.Sprintf("-%d", note.Position.OldLine)
		} else if note.Position.OldLine == 0 {
			key = fmt.Sprintf("+%d", note.Position.NewLine)
		} else {
			key = fmt.Sprintf(" %d_%d", note.Position.NewLine, note.Position.OldLine)
		}

		if _, ok := noteIndex[key]; !ok {
			noteIndex[key] = nil
		}

		noteIndex[key] = append(noteIndex[key], nidx)

	}

	f.lineMap[0] = FRHeader

	for lineIdx < len(f.ff.lines) {
		if abrIdx < len(f.abrs) && lineIdx == f.abrs[abrIdx].start {
			f.lineMap = append(f.lineMap, (abrIdx*NUM_FR_TYPES)+FRAbr)
			lineIdx = f.abrs[abrIdx].end + 1
			abrIdx++
		} else {
			f.lineMap = append(f.lineMap, (lineIdx*NUM_FR_TYPES)+FRLine)
			var key string
			formattedLine := f.ff.lines[lineIdx]

			if formattedLine.mode == ADDED {
				key = fmt.Sprintf("+%d", formattedLine.bNum)
			} else if formattedLine.mode == REMOVED {
				key = fmt.Sprintf("-%d", formattedLine.aNum)
			} else {
				key = fmt.Sprintf(" %d_%d", formattedLine.bNum, formattedLine.aNum)
			}

			if noteIndicies, ok := noteIndex[key]; ok {
				for _, nidx := range noteIndicies {
					note := f.notes[nidx]
					f.lineMap = append(f.lineMap, (nidx*NUM_FR_TYPES)+FRComment)
					commentHeight := note.Height(vp)
					for i := 1; i < commentHeight; i++ {
						f.lineMap = append(f.lineMap, FRBlank)
					}
				}
			}

			lineIdx++
		}
	}
}

func newFileRegion(ff *FormattedFile, change GLChangeData, notes []GLNote, width int) *FileRegion {
	region := FileRegion{
		ff:        ff,
		oldPath:   change.OldPath,
		newPath:   change.NewPath,
		added:     change.NewFile,
		removed:   change.DeletedFile,
		collapsed: change.DeletedFile,
		notes:     notes,
	}

	inNonAbr := ff.lines[0].mode != UNCHANGED
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
	region.updateLineMap(&ViewParams{
		lineNoColWidth: region.lineNoColWidth,
		width:          width,
	})
	return &region
}
