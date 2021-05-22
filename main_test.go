package main

import (
	"reflect"
	"testing"
)

func TestParseLinksFromMarkdown(t *testing.T) {
	var tests = []struct {
		name     string
		given    string
		expected []string
	}{
		{
			"single link",
			" [abc](https://example.com)",
			[]string{"https://example.com"},
		},
		{
			"multiple links",
			" [abc](https://example.com) [bcd](https://example.org)",
			[]string{"https://example.com", "https://example.org"},
		},
		{
			"http link",
			" [abc](http://example.com)",
			[]string{"http://example.com"},
		},
		{
			"invalid link",
			" [abc](http://)",
			nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLinksFromMarkdown(tt.given)
			if err != nil {
				t.Errorf("expected nil error, got %+v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("(%+v): expected %+v, got %+v", tt.given, tt.expected, result)
			}
		})
	}
}
