package main

import "testing"

func TestCheckValues(t *testing.T) {
	answer := [][]string{{"ALGERIA", "11398", "1674125244.0242"}, {"ARGENTINA", "11500", "1700000000.5"}}
	for _, tc := range []struct {
		name string
		got  [][]string
		ok   bool
	}{
		{"the same", answer, true},
		{"in another order", [][]string{answer[1], answer[0]}, true},
		{"numbers rounded to 9 decimals", [][]string{{"ALGERIA", "11398", "1674125244.024200000"}, answer[1]}, true},
		{"a fanned-out count", [][]string{{"ALGERIA", "45592", "1674125244.0242"}, answer[1]}, false},
		{"a revenue off by 0.01%", [][]string{{"ALGERIA", "11398", "1674292656.5"}, answer[1]}, false},
		{"a row missing", answer[:1], false},
		{"another name", [][]string{{"ALGERIa", "11398", "1674125244.0242"}, answer[1]}, false},
	} {
		if err := checkValues(answer, tc.got); (err == nil) != tc.ok {
			t.Errorf("%s: checkValues = %v, want ok %v", tc.name, err, tc.ok)
		}
	}

	months := [][]string{{"1992-01-01", "2812229777.7298"}}
	for _, written := range []string{"1992-01-01", "1992-01-01 00:00:00", "1992-01-01T00:00:00.000"} {
		if err := checkValues(months, [][]string{{written, "2812229777.7298"}}); err != nil {
			t.Errorf("month written %q: %v", written, err)
		}
	}
	if err := checkValues(months, [][]string{{"1992-02-01", "2812229777.7298"}}); err == nil {
		t.Error("another month passed")
	}
}
