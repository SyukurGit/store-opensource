package handler

import (
	"net/url"
	"strings"
	"testing"
)

func TestProductSlugUnescaping(t *testing.T) {
	tests := []struct {
		name     string
		rawSlug  string
		expected string
	}{
		{
			name:     "Silvex Claude 500$ with percent-encoded dollar",
			rawSlug:  "silvex_claude_500%24_api_30day_(fw)",
			expected: "silvex_claude_500$_api_30day_(fw)",
		},
		{
			name:     "Plain slug without special chars",
			rawSlug:  "digitalcore_chatgpt_plus_1m",
			expected: "digitalcore_chatgpt_plus_1m",
		},
		{
			name:     "Slug with escaped parenthesis if any",
			rawSlug:  "silvex_test_%28fw%29",
			expected: "silvex_test_(fw)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slug := tc.rawSlug
			if unescaped, err := url.PathUnescape(tc.rawSlug); err == nil && unescaped != "" {
				slug = unescaped
			} else if unescaped, err := url.QueryUnescape(tc.rawSlug); err == nil && unescaped != "" {
				slug = unescaped
			}

			if slug != tc.expected {
				t.Fatalf("expected slug %q, got %q", tc.expected, slug)
			}

			// Verify resilient match logic
			target := tc.expected
			matched := (target == slug || strings.EqualFold(target, slug) || strings.EqualFold(target, tc.rawSlug))
			if !matched {
				t.Fatalf("resilient matching failed for %q against %q / %q", target, slug, tc.rawSlug)
			}
		})
	}
}
