package console

import "testing"

func TestParamsZeroValueHasNoDepthLimit(t *testing.T) {
	var params Params
	if params.Depth != nil {
		t.Errorf("Depth = %v, want nil", params.Depth)
	}
}

func TestParamsStoresPathAndDepth(t *testing.T) {
	tests := []struct {
		name  string
		depth int
	}{
		{name: "zero_depth", depth: 0},
		{name: "positive_depth", depth: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const path = "/input/report.json"
			depth := tt.depth
			params := Params{Path: path, Depth: &depth}

			if params.Path != path {
				t.Errorf("Path = %q, want %q", params.Path, path)
			}
			if params.Depth == nil {
				t.Fatal("Depth = nil, want an explicit limit")
			}
			if *params.Depth != tt.depth {
				t.Errorf("Depth = %d, want %d", *params.Depth, tt.depth)
			}
		})
	}
}
