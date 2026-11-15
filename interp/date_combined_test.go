package interp

import (
	"testing"
	"time"
)

func TestParseDateHeaderCombinedForms(t *testing.T) {
	cases := []struct {
		value string
		want  time.Time
	}{
		{"Sat, 31 Dec (c) 2016 23 (h): 59 (m): 60 (s) +0000", time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"Wed, 17 Jul (y) 096 (h) 02:23:25 -0700", time.Date(1996, 7, 17, 9, 23, 25, 0, time.UTC)},
		{"by mx; Sat, 31 Dec (year: old) 116 23 (h): 59 (m): 60 GMT (last; hop)", time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC)},
	}
	for _, tc := range cases {
		got, err := parseDateHeader(tc.value)
		if err != nil || !got.Equal(tc.want) {
			t.Errorf("%q: got%v err%v want%v", tc.value, got, err, tc.want)
		}
	}
}
