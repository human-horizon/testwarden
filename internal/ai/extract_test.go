package ai

import "testing"

func TestExtractCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fenced block",
			input: "Here is the fix:\n```go\npackage foo\n\nfunc Bar() {}\n```\nDone.",
			want:  "package foo\n\nfunc Bar() {}",
		},
		{
			name:  "no fence",
			input: "package foo\n\nfunc Bar() {}",
			want:  "package foo\n\nfunc Bar() {}",
		},
		{
			name:  "fence without language",
			input: "```\nx := 1\n```",
			want:  "x := 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCode(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
