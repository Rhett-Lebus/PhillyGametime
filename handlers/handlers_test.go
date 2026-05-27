package handlers

import (
	"os"
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

func TestShouldShowThemePickerInMockMode(t *testing.T) {
	t.Setenv("PHILLY_DATA", "mock")
	t.Setenv("PHILLY_ENV", "")
	t.Setenv("PORT", "8081")

	if !shouldShowThemePicker() {
		t.Fatal("shouldShowThemePicker() = false, want true in mock mode")
	}
}

func TestShouldHideThemePickerInProduction(t *testing.T) {
	t.Setenv("PHILLY_DATA", "mock")
	t.Setenv("PHILLY_ENV", "production")
	t.Setenv("PORT", "8080")

	if shouldShowThemePicker() {
		t.Fatal("shouldShowThemePicker() = true, want false in production")
	}
}

func TestShouldShowThemePickerOnDefaultLocalPort(t *testing.T) {
	t.Setenv("PHILLY_DATA", "")
	t.Setenv("PHILLY_ENV", "")
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatal(err)
	}

	if !shouldShowThemePicker() {
		t.Fatal("shouldShowThemePicker() = false, want true with default local port")
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
