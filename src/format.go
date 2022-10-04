package main

import (
	"fmt"
	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	gloss "github.com/charmbracelet/lipgloss"
	"strings"
)

type UnRenderedToken struct {
	text  string
	style gloss.Style
}

type FormattedLine struct {
	tokens []UnRenderedToken
	mode   Mode
	aNum   int
	bNum   int
}

func (l *FormattedLine) Render(background gloss.Color) string {
	var b strings.Builder

	for _, token := range l.tokens {
		b.WriteString(token.style.Background(background).Render(token.text))
	}

	return b.String()
}

type FormattedFile struct {
	lines []*FormattedLine
}

func ReconstituteDiff(df *DiffFile) (string, string) {
	var aBuilder strings.Builder
	var bBuilder strings.Builder

	for _, line := range df.lines {
		if line.mode == UNCHANGED {
			aBuilder.WriteString(line.text)
			bBuilder.WriteString(line.text)
			aBuilder.WriteString("\n")
			bBuilder.WriteString("\n")
		} else if line.mode == REMOVED {
			aBuilder.WriteString(line.text)
			aBuilder.WriteString("\n")
		} else {
			bBuilder.WriteString(line.text)
			bBuilder.WriteString("\n")
		}
	}

	return aBuilder.String(), bBuilder.String()
}

func Highlight(s string, fType string) ([][]UnRenderedToken, error) {
	var ret [][]UnRenderedToken

	lexer := chroma.Coalesce(lexers.Get("javascript"))
	style := styles.Get("vim")
	detabbed := strings.ReplaceAll(s, "\t", "  ")
	ti, err := lexer.Tokenise(nil, detabbed)
	if err != nil {
		return nil, err
	}

	rawTokens := ti.Tokens()
	lines := chroma.SplitTokensIntoLines(rawTokens)

	for _, line := range lines {
		var unRenderedTokens []UnRenderedToken

		for _, token := range line {
			s := style.Get(token.Type)
			unRenderedTokens = append(unRenderedTokens, UnRenderedToken{
				style: gloss.NewStyle().Foreground(gloss.Color(fmt.Sprintf("#%X", int32(s.Colour)))),
				text:  strings.ReplaceAll(token.Value, "\n", ""),
			})
		}

		ret = append(ret, unRenderedTokens)
	}

	return ret, nil
}

func FormatFile(base string, diff string, fType string) (*FormattedFile, error) {
	var formattedFile FormattedFile

	df, err := AnnotateWithDiff(base, diff)
	if err != nil {
		return nil, err
	}

	baseStr, targStr := ReconstituteDiff(df)
	baseFormatted, err := Highlight(baseStr, fType)
	if err != nil {
		return nil, err
	}
	targFormatted, err := Highlight(targStr, fType)
	if err != nil {
		return nil, err
	}

	for _, line := range df.lines {
		var tokens []UnRenderedToken

		if line.mode == UNCHANGED {
			tokens = baseFormatted[line.aNum-1]
		} else if line.mode == REMOVED {
			tokens = baseFormatted[line.aNum-1]
		} else {
			tokens = targFormatted[line.bNum-1]
		}

		formattedFile.lines = append(formattedFile.lines, &FormattedLine{
			tokens: tokens,
			mode:   line.mode,
			aNum:   line.aNum,
			bNum:   line.bNum,
		})
	}

	return &formattedFile, nil
}
