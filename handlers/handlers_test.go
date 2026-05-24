package handlers

import (
	"testing"
	"time"

	"gametime/data"
	"gametime/events"
)

func TestFormatDateTimeUsesPhiladelphiaTime(t *testing.T) {
	h := New(data.NewMockStore(), events.NewBus())
	formatDateTime := h.funcMap["formatDateTime"].(func(time.Time) string)

	gameTime := time.Date(2026, time.May, 23, 20, 5, 0, 0, time.UTC)
	got := formatDateTime(gameTime)
	want := "Saturday, May 23 - 4:05 PM EDT"

	if got != want {
		t.Fatalf("formatDateTime() = %q, want %q", got, want)
	}
}

func TestDayLabelUsesPhiladelphiaTime(t *testing.T) {
	h := New(data.NewMockStore(), events.NewBus())
	dayLabel := h.funcMap["dayLabel"].(func(time.Time) string)

	now := data.NowPhilly()
	tomorrow := data.DatePhilly(now.Year(), now.Month(), now.Day()+1, 1, 0, 0)

	if got := dayLabel(tomorrow); got != "Tomorrow" {
		t.Fatalf("dayLabel() = %q, want Tomorrow", got)
	}
}

func TestRecapSentencesSplitsReadableSentences(t *testing.T) {
	got := recapSentences("Wheeler pitched six shutout innings. Stott drove in two runs.")
	want := []string{"Wheeler pitched six shutout innings.", "Stott drove in two runs."}

	if len(got) != len(want) {
		t.Fatalf("recapSentences() returned %d sentences, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recapSentences()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRecapSentencesBreaksLongCommaSeparatedRecap(t *testing.T) {
	got := recapSentences("Zack Wheeler pitched six shutout innings and Bryson Stott hit a two-run single as the Philadelphia Phillies defeated Cleveland 3-0 on Saturday, ending the Guardians' seven-game winning streak.")
	want := []string{
		"Zack Wheeler pitched six shutout innings and Bryson Stott hit a two-run single as the Philadelphia Phillies defeated Cleveland 3-0 on Saturday",
		"ending the Guardians' seven-game winning streak.",
	}

	if len(got) != len(want) {
		t.Fatalf("recapSentences() returned %d chunks, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recapSentences()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
