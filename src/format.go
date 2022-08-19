package main

import (
  "fmt"
  "strings"
  "github.com/alecthomas/chroma"
  "github.com/alecthomas/chroma/lexers"
  "github.com/alecthomas/chroma/styles"
  gloss "github.com/charmbracelet/lipgloss"
)

type UnRenderedToken struct {
  text string
  style gloss.Style
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
  style := styles.Get("monokai")
  ti, err := lexer.Tokenise(nil, s)
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
        text: strings.ReplaceAll(token.Value, "\n", ""),
      })
    }

    ret = append(ret, unRenderedTokens)
  }

  return ret, nil
}

func FormatFile(base string, diff string, fType string) (*DiffFile, error) {
  var formattedDiff DiffFile

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

  fmt.Printf("a %d b %d %s", len(baseFormatted), len(targFormatted), baseFormatted[0])

  for _, line := range df.lines {
    deRefLine := *line
    var builder strings.Builder

    if line.mode == UNCHANGED {
      for _, token := range baseFormatted[line.aNum - 1] {
        builder.WriteString(token.style.Faint(true).Render(token.text))
      }
    } else if line.mode == REMOVED {
      for _, token := range baseFormatted[line.aNum - 1] {
        builder.WriteString(token.style.Render(token.text))
      }
    } else {
      for _, token := range baseFormatted[line.bNum - 1] {
        builder.WriteString(token.style.Render(token.text))
      }
    }

    deRefLine.text = builder.String()
    formattedDiff.lines = append(formattedDiff.lines, &deRefLine)
    //fmt.Printf("%s\n", deRefLine.text)
  }

  return &formattedDiff, nil
}

