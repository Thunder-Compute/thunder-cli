package utils

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{name: "empty", input: "", want: nil},
		{name: "single", input: "8080", want: []int{8080}},
		{name: "list and range", input: "8080, 9000-9002", want: []int{8080, 9000, 9001, 9002}},
		{name: "range single", input: "8000-8000", want: []int{8000}},
		{name: "invalid range order", input: "9002-9000", wantErr: true},
		{name: "reserved single", input: "22", wantErr: true},
		{name: "reserved in range", input: "20-25", wantErr: true},
		{name: "out of range low", input: "0", wantErr: true},
		{name: "out of range high", input: "65536", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParsePorts(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePorts() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParsePorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatPorts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []int
		want  string
	}{
		{name: "empty", input: nil, want: ""},
		{name: "single", input: []int{8080}, want: "8080"},
		{name: "short run", input: []int{8000, 8001, 8002, 8003}, want: "8000, 8001, 8002, 8003"},
		{name: "range", input: []int{8000, 8001, 8002, 8003, 8004}, want: "8000-8004"},
		{name: "mixed", input: []int{8000, 8001, 8002, 9000, 9001, 9002, 9003, 9004, 9005}, want: "8000, 8001, 8002, 9000-9005"},
		{name: "unsorted dedupe", input: []int{9005, 9001, 9003, 9002, 9004, 9005}, want: "9001-9005"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FormatPorts(tt.input)
			if got != tt.want {
				t.Fatalf("FormatPorts() = %q, want %q", got, tt.want)
			}
		})
	}
}
