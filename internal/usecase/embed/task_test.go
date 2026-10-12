package embed

import "testing"

func TestTextPrefix(t *testing.T) {
	cases := []struct {
		task      string
		inputType string
		title     string
		want      string
	}{
		{"search_query", "", "", " task: search result | query: "},
		{"code_retrieval", "", "", " task: code retrieval | query: "},
		{"", "query", "", " task: search result | query: "},
		{"", "document", "", " title: none | text: "},
		{"", "document", "Kumbukumbu", " title: Kumbukumbu | text: "},
		{"", "", "", ""},
		{"unknown_task", "", "", ""},
		{"search_query", "document", "ignored", " task: search result | query: "},
	}
	for _, c := range cases {
		if got := textPrefix(c.task, c.inputType, c.title); got != c.want {
			t.Errorf("textPrefix(%q,%q,%q) = %q, want %q", c.task, c.inputType, c.title, got, c.want)
		}
	}
}
