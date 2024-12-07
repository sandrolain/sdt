package cmd

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseCsvArray(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("separator", ",", "CSV separator")
	cmd.Flags().Bool("object", false, "Parse as object")

	input := `a,b,c
1,2,3
4,5,6`

	expected := [][]string{
		{"a", "b", "c"},
		{"1", "2", "3"},
		{"4", "5", "6"},
	}

	result := parseCsv(cmd, input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseCsvObject(t *testing.T) {
	cmd := &cobra.Command{}
	_ = cmd.Flags().String("separator", ",", "CSV separator")
	_ = cmd.Flags().Bool("object", true, "Parse as object")
	_ = cmd.Flags().Set("object", "true")

	input := `header1,header2,header3
value1,value2,value3
value4,value5,value6`

	expected := []map[string]string{
		{
			"header1": "value1",
			"header2": "value2",
			"header3": "value3",
		},
		{
			"header1": "value4",
			"header2": "value5",
			"header3": "value6",
		},
	}

	result := parseCsv(cmd, input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseCsvCustomSeparator(t *testing.T) {
	cmd := &cobra.Command{}
	_ = cmd.Flags().String("separator", ";", "CSV separator")
	_ = cmd.Flags().Bool("object", false, "Parse as object")
	_ = cmd.Flags().Set("separator", ";")

	input := `a;b;c
1;2;3
4;5;6`

	expected := [][]string{
		{"a", "b", "c"},
		{"1", "2", "3"},
		{"4", "5", "6"},
	}

	result := parseCsv(cmd, input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestBuildCsv(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("separator", ",", "CSV separator")

	input := []interface{}{
		[]interface{}{"header1", "header2", "header3"},
		[]interface{}{"value1", "value2", "value3"},
		[]interface{}{"value4", "value5", "value6"},
	}

	expected := "header1,header2,header3\nvalue1,value2,value3\nvalue4,value5,value6\n"

	result, err := buildCsv(cmd, input)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected %q, got %q", expected, string(result))
	}
}

func TestBuildCsvCustomSeparator(t *testing.T) {
	cmd := &cobra.Command{}
	_ = cmd.Flags().String("separator", ";", "CSV separator")
	_ = cmd.Flags().Set("separator", ";")

	input := []interface{}{
		[]interface{}{"header1", "header2", "header3"},
		[]interface{}{"value1", "value2", "value3"},
	}

	expected := "header1;header2;header3\nvalue1;value2;value3\n"

	result, err := buildCsv(cmd, input)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected %q, got %q", expected, string(result))
	}
}

func TestBuildCsvErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("separator", ",", "CSV separator")

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "Not an array",
			input:   "not an array",
			wantErr: true,
		},
		{
			name: "Row not an array",
			input: []interface{}{
				"not an array",
			},
			wantErr: true,
		},
		{
			name: "Different row lengths",
			input: []interface{}{
				[]interface{}{"a", "b"},
				[]interface{}{"c"},
			},
			wantErr: true,
		},
		{
			name: "Non-string value",
			input: []interface{}{
				[]interface{}{"a", 123},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildCsv(cmd, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildCsv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
