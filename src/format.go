package main

import (
  "fmt"
  "strings"
  "github.com/alecthomas/chroma"
  "github.com/alecthomas/chroma/lexers"
  "github.com/alecthomas/chroma/styles"
  gloss "github.com/charmbracelet/lipgloss"
)

/*
type FormattedFile {
}

func formatFile(base string, diff string, fType string) (*FormattedFile, error) {
  df, err := AnnotateWithDiff(string(base), string(diff))
  if err != nil {
    panic(err)
  }

  model := Model{
    diff: df,
    lineNoColWidth: GetLineNoColWidth(df),
  }

  return model

}

*/

func TestFormat(s string) {
  var builder strings.Builder

  lexer := chroma.Coalesce(lexers.Get("javascript"))
  style := styles.Get("monokai")
  ti, err := lexer.Tokenise(nil, s)
  if err != nil {
    panic(err)
  }

  lines := chroma.SplitTokensIntoLines(ti.Tokens())

  for _, line := range lines {
    for _, token := range line {
      s := style.Get(token.Type)
      gStyle := gloss.NewStyle().Foreground(gloss.Color(fmt.Sprintf("#%X\n", int32(s.Colour))))
      builder.WriteString(gStyle.Render(token.Value))
    }
  }
  fmt.Println(builder.String())
}
