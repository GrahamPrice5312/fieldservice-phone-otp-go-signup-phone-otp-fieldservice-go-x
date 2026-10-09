package main

import "testing"

func TestCanDispatch(t *testing.T) {
	base := WorkOrder{ID: "wo-17", Technician: "tech-4", Status: "ready"}
	tests := []struct {
		name     string
		verified bool
		order    WorkOrder
		want     bool
	}{
		{"verified ready order", true, base, true},
		{"unverified technician", false, base, false},
		{"order still queued", true, WorkOrder{ID: "wo-17", Technician: "tech-4", Status: "queued"}, false},
		{"missing technician", true, WorkOrder{ID: "wo-17", Status: "ready"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canDispatch(tt.verified, tt.order); got != tt.want {
				t.Fatalf("canDispatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
