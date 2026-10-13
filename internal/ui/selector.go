package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	internalaws "github.com/inamuu/ssmx/internal/aws"
	"github.com/ktr0731/go-fuzzyfinder/matching"
	"github.com/mattn/go-runewidth"
)

// ErrAbort is returned when the user cancels the selection.
var ErrAbort = errors.New("abort")

const maxNamePad = 48

var newScreen = tcell.NewScreen

func SelectInstance(instances []internalaws.Instance) (*internalaws.Instance, error) {
	targets := make([]internalaws.SessionTarget, 0, len(instances))
	for _, inst := range instances {
		targets = append(targets, internalaws.SessionTarget{
			Kind:      internalaws.SessionTargetKindEC2,
			TargetID:  inst.InstanceID,
			Name:      inst.Name,
			PrivateIP: inst.PrivateIP,
		})
	}

	idx, err := selectTarget(targets)
	if err != nil {
		return nil, err
	}
	return &instances[idx], nil
}

func SelectSessionTarget(targets []internalaws.SessionTarget) (*internalaws.SessionTarget, error) {
	idx, err := selectTarget(targets)
	if err != nil {
		return nil, err
	}
	return &targets[idx], nil
}

func selectTarget(targets []internalaws.SessionTarget) (int, error) {
	namePad := 0
	for _, t := range targets {
		if w := runewidth.StringWidth(displayName(t)); w > namePad {
			namePad = w
		}
	}
	if namePad > maxNamePad {
		namePad = maxNamePad
	}

	labels := make([]string, len(targets))
	for i, t := range targets {
		labels[i] = targetLabel(t, namePad)
	}

	return selectItem(labels, func(i int) string { return targets[i].DetailText() })
}

func displayName(t internalaws.SessionTarget) string {
	if t.Name != "" {
		return t.Name
	}
	return "-"
}

func targetLabel(t internalaws.SessionTarget, namePad int) string {
	name := runewidth.FillRight(displayName(t), namePad)
	if t.Kind == internalaws.SessionTargetKindECS {
		return fmt.Sprintf("[ECS] %s  container:%s  cluster:%s  task:%s", name, t.ContainerName, t.ClusterName, t.TaskID)
	}
	return fmt.Sprintf("[EC2] %s  %s  %s", name, t.TargetID, t.PrivateIP)
}

// selectItem shows an interactive fuzzy finder. The item list is drawn on the
// upper side and the detail of the current item on the lower side, so long
// labels can use the full terminal width.
func selectItem(labels []string, detail func(i int) string) (int, error) {
	screen, err := newScreen()
	if err != nil {
		return -1, err
	}
	if err := screen.Init(); err != nil {
		return -1, err
	}
	defer screen.Fini()

	s := &selector{screen: screen, labels: labels, detail: detail}
	s.filter()

	for {
		s.draw()
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc, tcell.KeyCtrlC, tcell.KeyCtrlD:
				return -1, ErrAbort
			case tcell.KeyEnter:
				if len(s.matched) == 0 {
					continue
				}
				return s.matched[s.cursor], nil
			case tcell.KeyUp, tcell.KeyCtrlP, tcell.KeyCtrlK:
				s.move(-1)
			case tcell.KeyDown, tcell.KeyCtrlN, tcell.KeyCtrlJ, tcell.KeyTab:
				s.move(1)
			case tcell.KeyPgUp:
				s.move(-s.listHeight())
			case tcell.KeyPgDn:
				s.move(s.listHeight())
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(s.query) > 0 {
					s.query = s.query[:len(s.query)-1]
					s.filter()
				}
			case tcell.KeyCtrlU:
				s.query = nil
				s.filter()
			case tcell.KeyCtrlW:
				q := strings.TrimRight(string(s.query), " ")
				if i := strings.LastIndex(q, " "); i >= 0 {
					s.query = []rune(q[:i+1])
				} else {
					s.query = nil
				}
				s.filter()
			case tcell.KeyRune:
				s.query = append(s.query, ev.Rune())
				s.filter()
			}
		}
	}
}

type selector struct {
	screen  tcell.Screen
	labels  []string
	detail  func(i int) string
	query   []rune
	matched []int
	cursor  int
	offset  int
}

func (s *selector) filter() {
	s.matched = s.matched[:0]
	q := strings.TrimSpace(string(s.query))
	if q == "" {
		for i := range s.labels {
			s.matched = append(s.matched, i)
		}
	} else {
		for _, m := range matching.FindAll(q, s.labels) {
			s.matched = append(s.matched, m.Idx)
		}
	}
	s.cursor = 0
	s.offset = 0
}

func (s *selector) move(delta int) {
	if len(s.matched) == 0 {
		return
	}
	s.cursor += delta
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= len(s.matched) {
		s.cursor = len(s.matched) - 1
	}
}

func (s *selector) detailLines() []string {
	if len(s.matched) == 0 {
		return nil
	}
	return strings.Split(s.detail(s.matched[s.cursor]), "\n")
}

// listHeight is the number of rows available for items: everything except
// the prompt, the status line, the separator and the detail pane.
func (s *selector) listHeight() int {
	_, h := s.screen.Size()
	n := h - 3 - len(s.detailLines())
	if n < 1 {
		n = 1
	}
	return n
}

func (s *selector) draw() {
	s.screen.Clear()
	w, _ := s.screen.Size()

	normal := tcell.StyleDefault
	dim := normal.Foreground(tcell.ColorGray)
	selected := normal.Bold(true).Reverse(true)
	accent := normal.Foreground(tcell.ColorTeal).Bold(true)

	row := 0
	prompt := "> " + string(s.query)
	putString(s.screen, 0, row, w, prompt, accent)
	s.screen.ShowCursor(runewidth.StringWidth(prompt), row)
	row++

	putString(s.screen, 0, row, w, fmt.Sprintf("  %d/%d  (Enter: select, Esc: cancel)", len(s.matched), len(s.labels)), dim)
	row++

	height := s.listHeight()
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+height {
		s.offset = s.cursor - height + 1
	}
	for i := 0; i < height; i++ {
		idx := s.offset + i
		if idx >= len(s.matched) {
			break
		}
		style := normal
		marker := "  "
		if idx == s.cursor {
			style = selected
			marker = "> "
		}
		line := marker + s.labels[s.matched[idx]]
		putString(s.screen, 0, row+i, w, runewidth.FillRight(line, w), style)
	}
	row += height

	putString(s.screen, 0, row, w, strings.Repeat("─", w), dim)
	row++
	for i, line := range s.detailLines() {
		putString(s.screen, 0, row+i, w, line, normal)
	}

	s.screen.Show()
}

func putString(screen tcell.Screen, x, y, maxWidth int, str string, style tcell.Style) {
	for _, r := range str {
		rw := runewidth.RuneWidth(r)
		if x+rw > maxWidth {
			return
		}
		screen.SetContent(x, y, r, nil, style)
		x += rw
	}
}
