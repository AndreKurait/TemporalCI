package eval

import "testing"

func TestExpandMatrix_30Elements(t *testing.T) {
	// Simulates opensearch-migrations 30-way Gradle striping
	indices := make([]string, 30)
	for i := range indices {
		indices[i] = string(rune('0' + i/10)) + string(rune('0' + i%10))
	}
	combos := ExpandMatrix(map[string][]string{"index": indices}, nil, nil)
	if len(combos) != 30 {
		t.Fatalf("expected 30 combos, got %d", len(combos))
	}
	// Verify each combo has the correct index
	for i, c := range combos {
		if c["index"] != indices[i] {
			t.Errorf("combo[%d] index = %q, want %q", i, c["index"], indices[i])
		}
	}
}

func TestExpandMatrix_LargeCartesian(t *testing.T) {
	// 3 arches x 2 OSes x 5 versions = 30 combos
	combos := ExpandMatrix(map[string][]string{
		"arch":    {"x86", "arm", "riscv"},
		"os":      {"linux", "mac"},
		"version": {"1", "2", "3", "4", "5"},
	}, nil, nil)
	if len(combos) != 30 {
		t.Fatalf("expected 30 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_ExcludeAndInclude(t *testing.T) {
	combos := ExpandMatrix(
		map[string][]string{"arch": {"x86", "arm"}, "os": {"linux", "mac", "win"}},
		[]map[string]string{{"arch": "arm", "os": "win"}},                          // exclude arm+win
		[]map[string]string{{"arch": "riscv", "os": "linux", "special": "true"}},   // include extra
	)
	// 2*3=6 - 1 exclude + 1 include = 6
	if len(combos) != 6 {
		t.Fatalf("expected 6 combos, got %d", len(combos))
	}
}

func TestMatrixKey_SingleDimension(t *testing.T) {
	key := MatrixKey(map[string]string{"index": "5"})
	if key != "index=5" {
		t.Errorf("got %q", key)
	}
}
