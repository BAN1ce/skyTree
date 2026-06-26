package topic

import "testing"

func TestValidateTopicFilterSyntax(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		valid  bool
	}{
		{name: "plain topic", filter: "a/b/c", valid: true},
		{name: "single wildcard segment", filter: "a/+/c", valid: true},
		{name: "multi wildcard final", filter: "a/#", valid: true},
		{name: "shared topic", filter: "$share/g/a/+/c", valid: true},
		{name: "empty filter", filter: "", valid: false},
		{name: "invalid embedded plus", filter: "a+b/c", valid: false},
		{name: "invalid embedded hash", filter: "a#b/c", valid: false},
		{name: "invalid hash not final", filter: "a/#/c", valid: false},
		{name: "shared missing group", filter: "$share//a/b", valid: false},
		{name: "shared missing filter", filter: "$share/g/", valid: false},
		{name: "shared group wildcard", filter: "$share/g+/a/b", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTopicFilterSyntax(tt.filter)
			if tt.valid && err != nil {
				t.Fatalf("expected valid filter, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}
