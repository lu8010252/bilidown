package util

import ("testing"; "unicode/utf8")

func TestTruncateBytes(t *testing.T) {
	s := "名某难的日常 [1] 只用一个表情就被全校通缉了 [想设个名字就这么难吗] [高清 1080P60] [1分6秒]"
	for _, max := range []int{1, 2, 3, 4, 7, 20, 50, 200, 1000} {
		out := TruncateBytes(s, max)
		if len(out) > max || !utf8.ValidString(out) || (max >= len(s) && out != s) {
			t.Fatalf("max=%d bad: len=%d valid=%v", max, len(out), utf8.ValidString(out))
		}
	}
	if TruncateBytes("abc", 10) != "abc" { t.Fatal("short string changed") }
}
