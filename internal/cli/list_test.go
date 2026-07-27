package cli

import (
	"strings"
	"testing"
)

func TestBoxchar(t *testing.T) {
	tests := []struct {
		name     string
		pos      int // position in list
		level    int // nesting level
		bound    int // total items (boundary)
		expected string
	}{
		{
			name:     "first item at root level",
			pos:      0,
			level:    0,
			bound:    5,
			expected: "┌─",
		},
		{
			name:     "middle item at root level",
			pos:      2,
			level:    0,
			bound:    5,
			expected: "├─",
		},
		{
			name:     "last item at root level",
			pos:      5,
			level:    0,
			bound:    5,
			expected: "└─",
		},
		{
			name:     "first item in nested level",
			pos:      0,
			level:    1,
			bound:    3,
			expected: "└─",
		},
		{
			name:     "single item (pos == bound == 0)",
			pos:      0,
			level:    0,
			bound:    0,
			expected: "└─", // When pos == bound, it's treated as the last item
		},
		{
			name:     "nested level with pos > 0",
			pos:      1,
			level:    1,
			bound:    3,
			expected: "├─",
		},
		{
			name:     "deeply nested first item",
			pos:      0,
			level:    3,
			bound:    2,
			expected: "└─",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boxchar(tt.pos, tt.level, tt.bound)
			if result != tt.expected {
				t.Errorf("boxchar(%d, %d, %d) = %q, want %q",
					tt.pos, tt.level, tt.bound, result, tt.expected)
			}
		})
	}
}

func TestBoxchar_ContainsTreeCharacters(t *testing.T) {
	// All results should contain tree-drawing characters
	treeChars := []string{"┌", "├", "└", "─", "│"}

	testCases := []struct {
		pos, level, bound int
	}{
		{0, 0, 5},
		{2, 0, 5},
		{5, 0, 5},
		{0, 1, 3},
		{1, 1, 3},
	}

	for _, tc := range testCases {
		result := boxchar(tc.pos, tc.level, tc.bound)

		hasTreeChar := false
		for _, char := range treeChars {
			if strings.Contains(result, char) {
				hasTreeChar = true
				break
			}
		}

		if !hasTreeChar {
			t.Errorf("boxchar(%d, %d, %d) = %q, should contain tree characters",
				tc.pos, tc.level, tc.bound, result)
		}
	}
}

func TestBoxchar_FirstAndLast(t *testing.T) {
	// First item should have "┌" at root level
	first := boxchar(0, 0, 5)
	if !strings.HasPrefix(first, "┌") {
		t.Errorf("First item at root should start with ┌, got %q", first)
	}

	// Last item should have "└"
	last := boxchar(5, 0, 5)
	if !strings.HasPrefix(last, "└") {
		t.Errorf("Last item should start with └, got %q", last)
	}

	// Middle item should have "├"
	middle := boxchar(2, 0, 5)
	if !strings.HasPrefix(middle, "├") {
		t.Errorf("Middle item should start with ├, got %q", middle)
	}
}

func TestBoxchar_NestedLevels(t *testing.T) {
	// First item in nested level should have "└"
	nested := boxchar(0, 1, 3)
	if !strings.HasPrefix(nested, "└") {
		t.Errorf("First nested item should start with └, got %q", nested)
	}
}

func TestBoxchar_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name        string
		pos, level  int
		bound       int
		shouldStart string
	}{
		{"pos equals bound", 3, 0, 3, "└"},
		{"pos is zero, bound is zero", 0, 0, 0, "└"}, // pos == bound means last item
		{"pos greater than zero, less than bound", 1, 0, 3, "├"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boxchar(tt.pos, tt.level, tt.bound)
			if !strings.HasPrefix(result, tt.shouldStart) {
				t.Errorf("boxchar(%d, %d, %d) = %q, should start with %q",
					tt.pos, tt.level, tt.bound, result, tt.shouldStart)
			}
		})
	}
}
