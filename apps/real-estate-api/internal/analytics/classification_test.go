package analytics

import "testing"

func TestClassifySalePackage(t *testing.T) {
	tests := []struct {
		name      string
		buildings []int
		land      []int
		want      string
	}{
		{"house with land and garage", []int{1, 4}, []int{5}, "house"},
		{"apartment with parking", []int{2, 3}, nil, "apartment"},
		{"several apartments", []int{2, 2}, nil, "apartment"},
		{"mixed primary uses", []int{1, 2}, nil, "mixed"},
		{"commercial with land", []int{8}, []int{5}, "commercial"},
		{"garage only", []int{4}, nil, "garage"},
		{"land only", nil, []int{7}, "land"},
		{"unknown building", []int{99}, nil, "unknown"},
		{"no components", nil, nil, "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifySalePackage(test.buildings, test.land); got != test.want {
				t.Fatalf("ClassifySalePackage()=%q want=%q", got, test.want)
			}
		})
	}
}

func TestClassifyRentalPackage(t *testing.T) {
	tests := []struct {
		name  string
		types []int
		want  string
	}{
		{"one apartment", []int{2}, "apartment"},
		{"several apartments", []int{2, 2}, "apartment"},
		{"residential room", []int{16}, "room"},
		{"commercial spaces", []int{5, 8}, "commercial"},
		{"apartment and garage", []int{2, 4}, "mixed"},
		{"unknown", []int{15}, "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyRentalPackage(test.types); got != test.want {
				t.Fatalf("ClassifyRentalPackage()=%q want=%q", got, test.want)
			}
		})
	}
}
