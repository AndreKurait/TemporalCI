package activities

import "testing"

func TestExpandTemplateVars(t *testing.T) {
	tests := []struct {
		input string
		vars  map[string]string
		want  string
	}{
		{
			"./gradlew test -Dindex=${{ matrix.index }}",
			map[string]string{"index": "5"},
			"./gradlew test -Dindex=5",
		},
		{
			"echo ${{ matrix.arch }}-${{ matrix.os }}",
			map[string]string{"arch": "x86", "os": "linux"},
			"echo x86-linux",
		},
		{
			"no templates here",
			map[string]string{"x": "y"},
			"no templates here",
		},
		{
			"${{ matrix.missing }}",
			map[string]string{},
			"${{ matrix.missing }}",
		},
	}
	for _, tt := range tests {
		got := expandTemplateVars(tt.input, tt.vars)
		if got != tt.want {
			t.Errorf("expandTemplateVars(%q, %v) = %q, want %q", tt.input, tt.vars, got, tt.want)
		}
	}
}

func TestMaskSecrets(t *testing.T) {
	secrets := map[string]string{"TOKEN": "secret123", "KEY": "abcdef"}
	got := MaskSecrets("token is secret123 and key is abcdef", secrets)
	if got != "token is *** and key is ***" {
		t.Errorf("MaskSecrets = %q", got)
	}
}

func TestMaskSecrets_ShortValues(t *testing.T) {
	// Values <= 3 chars are not masked
	got := MaskSecrets("val is abc", map[string]string{"K": "abc"})
	if got != "val is abc" {
		t.Errorf("short values should not be masked, got %q", got)
	}
}
