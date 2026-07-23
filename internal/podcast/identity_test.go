package podcast

import "testing"

func TestIsItemID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{id: "podcast_35e53eae4704e19af844b851", want: true},
		{id: "external-podcast-35e53eae4704e19af844b851", want: true},
		{id: "podcast-external-35e53eae4704e19af844b851", want: true},
		{id: "external-track-35e53eae4704e19af844b851", want: false},
		{id: "", want: false},
	}

	for _, test := range tests {
		if got := IsItemID(test.id); got != test.want {
			t.Fatalf("IsItemID(%q) = %v, want %v", test.id, got, test.want)
		}
	}
}
