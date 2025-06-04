package strictjson

import (
	"testing"
)

type TestStruct struct {
	Name string
	Age  int `json:"aGe"`
}

func TestUnmarshalStrict(t *testing.T) {
	tests := []struct {
		Name        string
		input       string
		expected    TestStruct
		expectError bool
		errorMsg    string
	}{
		{
			Name:        "Valid JSON",
			input:       `{"Name": "John", "aGe": 30}`,
			expected:    TestStruct{Name: "John", Age: 30},
			expectError: false,
		},
		{
			Name:        "Extra field",
			input:       `{"Name": "John", "aGe": 30, "extra": "field"}`,
			expectError: true,
			errorMsg:    "Unexpected field: extra",
		},
		{
			Name:        "Missing field",
			input:       `{"Name": "John"}`,
			expectError: true,
			errorMsg:    "Missing field: aGe",
		},
		{
			Name:        "Invalid JSON type string instead of int",
			input:       `{"Name": "John", "aGe": "thirty"}`,
			expectError: true,
		},
		{
			Name:        "Invalid JSON type unquoted string",
			input:       `{"Name": John, "aGe": 30}`,
			expectError: true,
		},
		{
			Name:        "Incorrect case",
			input:       `{"Name": "John", "age": 30}`,
			expectError: true,
			errorMsg:    "Unexpected field: age",
		},
		{
			Name:        "Malformed JSON",
			input:       `{"Name": "John", "aGe": 30`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var result TestStruct
			err := Unmarshal([]byte(tt.input), &result)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error message '%s', but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected result %+v, but got %+v", tt.expected, result)
				}
			}
		})
	}
}

func TestUnmarshalStrict_EmptyJSON(t *testing.T) {
	input := `{}`
	var result TestStruct
	err := Unmarshal([]byte(input), &result)
	if err == nil {
		t.Errorf("expected error for missing fields, but got none")
	}
}

func TestUnmarshalStrict_NilStruct(t *testing.T) {
	input := `{}`
	err := Unmarshal([]byte(input), nil)
	if err == nil {
		t.Errorf("expected error for nil struct, but got none")
	}
}
