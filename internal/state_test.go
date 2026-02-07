package internal

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestNewDaemonState(t *testing.T) {
	state := NewDaemonState()
	must.NotNil(t, state)
	must.False(t, state.Stats.StartTime.IsZero())
	must.Eq(t, 0, state.Stats.TotalProcessed)
}

func TestIsKnown(t *testing.T) {
	state := NewDaemonState()

	must.False(t, state.IsKnown("test.mp3"))

	state.MarkProcessed("test.mp3", true)
	must.True(t, state.IsKnown("test.mp3"))
	must.False(t, state.IsKnown("other.mp3"))
}

func TestMarkProcessed(t *testing.T) {
	state := NewDaemonState()

	state.MarkProcessed("success.mp3", true)
	state.MarkProcessed("failed.mp3", false)
	state.MarkProcessed("success2.mp3", true)

	must.Eq(t, 3, state.Stats.TotalProcessed)
	must.Eq(t, 2, state.Stats.TotalSuccess)
	must.Eq(t, 1, state.Stats.TotalFailed)

	must.True(t, state.IsKnown("success.mp3"))
	must.True(t, state.IsKnown("failed.mp3"))
	must.True(t, state.IsKnown("success2.mp3"))
}

func TestGetStats(t *testing.T) {
	state := NewDaemonState()

	state.MarkProcessed("a.mp3", true)
	state.MarkProcessed("b.mp3", false)

	stats := state.GetStats()
	must.Eq(t, 2, stats.TotalProcessed)
	must.Eq(t, 1, stats.TotalSuccess)
	must.Eq(t, 1, stats.TotalFailed)
	must.False(t, stats.StartTime.IsZero())
}
