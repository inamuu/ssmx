package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	internalaws "github.com/inamuu/ssmx/internal/aws"
)

func TestSelectSessionTarget_LayoutAndFilter(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	newScreen = func() (tcell.Screen, error) { return sim, nil }
	defer func() { newScreen = tcell.NewScreen }()

	longName := "very-long-instance-name-for-production-batch-worker-01"
	targets := []internalaws.SessionTarget{
		{Kind: internalaws.SessionTargetKindEC2, TargetID: "i-0123456789abcdef0", Name: longName, PrivateIP: "10.0.0.1"},
		{Kind: internalaws.SessionTargetKindECS, TargetID: "arn", Name: "api-service", ClusterName: "prod", TaskID: "abc123", ContainerName: "app"},
	}

	done := make(chan *internalaws.SessionTarget)
	go func() {
		got, _ := SelectSessionTarget(targets)
		done <- got
	}()

	// wait until the first frame is drawn
	waitFor(t, sim, func(s string) bool { return strings.Contains(s, longName) })
	screen := dump(sim)
	t.Log("\n" + screen)
	if !strings.Contains(screen, "[EC2] "+longName) || !strings.Contains(screen, "[ECS] api-service") {
		t.Fatalf("list does not contain full labels:\n%s", screen)
	}
	if !strings.Contains(screen, "Instance ID: i-0123456789abcdef0") {
		t.Fatalf("detail pane missing:\n%s", screen)
	}

	for _, r := range "ecs" {
		sim.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
	sim.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)

	got := <-done
	if got == nil || got.Kind != internalaws.SessionTargetKindECS {
		t.Fatalf("selected %+v, want ECS target", got)
	}
}

func dump(sim tcell.SimulationScreen) string {
	cells, w, h := sim.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) > 0 {
				b.WriteRune(c.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func waitFor(t *testing.T, sim tcell.SimulationScreen, ok func(string) bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok(dump(sim)) {
			return
		}
		timeSleep()
	}
	t.Fatalf("screen never matched:\n%s", dump(sim))
}

func timeSleep() { time.Sleep(10 * time.Millisecond) }
