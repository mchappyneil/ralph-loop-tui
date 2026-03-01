package screens

import (
	"strings"
	"testing"
)

func TestRenderTaskPanel_HubEnabled(t *testing.T) {
	data := AnalyticsData{
		HubURL:        "https://hub.example.com",
		HubInstanceID: "my-repo/BD-1",
	}
	got := renderTaskPanel(data)
	if !strings.Contains(got, "enabled") {
		t.Error("expected 'enabled' when HubURL is set")
	}
	if !strings.Contains(got, "hub.example.com") {
		t.Error("expected hub URL in output")
	}
	if !strings.Contains(got, "my-repo/BD-1") {
		t.Error("expected instance ID in output")
	}
}

func TestRenderTaskPanel_HubDisabled(t *testing.T) {
	data := AnalyticsData{}
	got := renderTaskPanel(data)
	if !strings.Contains(got, "disabled") {
		t.Error("expected 'disabled' when HubURL is empty")
	}
	if strings.Contains(got, "enabled") {
		t.Error("should not contain 'enabled' when hub is disabled")
	}
}
