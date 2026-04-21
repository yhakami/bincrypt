package main

import "testing"

func TestSanitizePathForLogsRedactsPasteIDs(t *testing.T) {
	pasteID := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO12"

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "viewer paste path",
			path: "/p/" + pasteID,
			want: "/p/{paste_id}",
		},
		{
			name: "api paste path",
			path: "/api/paste/" + pasteID,
			want: "/api/paste/{paste_id}",
		},
		{
			name: "invalid short id remains visible for debugging",
			path: "/api/paste/short",
			want: "/api/paste/short",
		},
		{
			name: "non paste path",
			path: "/api/health",
			want: "/api/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePathForLogs(tt.path)
			if got != tt.want {
				t.Fatalf("sanitizePathForLogs(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
