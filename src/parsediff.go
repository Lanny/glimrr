package main

import (
  "fmt"
  "strconv"
  "regexp"
  "strings"
)

type Mode int
const (
  UNCHANGED Mode = 0
  ADDED          = 1
  REMOVED        = 2
)

type DiffLine struct {
  text string
  mode Mode
  aNum int
  bNum int
}

type Hunk struct {
  baseStart int
  baseEnd int
  lines []*DiffLine
}

type DiffFile struct {
  lines []*DiffLine
}

func parseHunks(diffLines []string) ([]*Hunk, error) {
  var hunks []*Hunk
  var hunk *Hunk
  var aLine int
  var bLine int
  var err error

  hunkPat := regexp.MustCompile(`@@ -(?P<AStart>\d+),(?P<ALen>\d+) \+(?P<BStart>\d+),(?P<BEnd>\d+) @@ (?P<Ctx>.*)`)

  for idx, line := range diffLines {
    lineNo := idx + 1
    switch {
    case line == "":
      continue
    case strings.HasPrefix(line, "@"):
      matches := hunkPat.FindStringSubmatch(line)

      if (len(matches) < 6) {
        return nil, fmt.Errorf("Unable to parse hunk header at line %d", lineNo)
      }

      aLine, err = strconv.Atoi(matches[1])
      if err != nil {
        return nil, fmt.Errorf("Unable to parse hunk start line at line %d", lineNo)
      }

      bLine, err = strconv.Atoi(matches[3])
      if err != nil {
        return nil, fmt.Errorf("Unable to parse hunk start line at line %d", lineNo)
      }

      if (hunk != nil) {
        hunks = append(hunks, hunk)
      }

      hunk = &Hunk{
        baseStart: aLine,
      }


    case strings.HasPrefix(line, "+"):
      line := DiffLine {
        text: line[1:],
        mode: ADDED,
        aNum: aLine,
        bNum: bLine,
      }
      hunk.lines = append(hunk.lines, &line)
      bLine++
    case strings.HasPrefix(line, "-"):
      line := DiffLine {
        text: line[1:],
        mode: REMOVED,
        aNum: aLine,
        bNum: bLine,
      }
      hunk.lines = append(hunk.lines, &line)
      aLine++
    case strings.HasPrefix(line, " "):
      line := DiffLine {
        text: line[1:],
        mode: UNCHANGED,
        aNum: aLine,
        bNum: bLine,
      }
      hunk.lines = append(hunk.lines, &line)

      aLine++
      bLine++
    default:
      return nil, fmt.Errorf("Unable to parse line %d", lineNo)
    }
  }

  return hunks, nil
}

func AnnotateWithDiff(base string, diff string) (*DiffFile, error) {
  var nextHunk *Hunk
  var diffFile DiffFile

  baseLines := strings.Split(base, "\n")
  hunks, err := parseHunks(strings.Split(diff, "\n"))
  aLine := 1
  bLine := 1
  hunkIdx := 0

  if err != nil {
    return nil, err
  }

  if (len(hunks) > 0) {
    nextHunk = hunks[0]
  }

  for {
    if aLine > len(baseLines) && nextHunk == nil {
      break
    }

    if nextHunk != nil && aLine >= nextHunk.baseStart {
      for _, diffLine := range nextHunk.lines {
        diffFile.lines = append(diffFile.lines, diffLine)

        aLine = diffLine.aNum
        bLine = diffLine.bNum
      }

      hunkIdx++
      if hunkIdx < len(hunks) {
        nextHunk = hunks[hunkIdx]
      } else {
        nextHunk = nil
      }
    } else {
      diffFile.lines = append(diffFile.lines, &DiffLine{
        text: baseLines[aLine - 1],
        mode: UNCHANGED,
        aNum: aLine,
        bNum: bLine,
      })
    }

    aLine++
    bLine++
  }

  return &diffFile, nil
}

