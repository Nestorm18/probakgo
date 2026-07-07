package webhandlers

import "testing"

func TestSetDotEnvValue(t *testing.T) {
	got := setDotEnvValue("SESSION_KEY=abc\nSESSION_SECURE=false\nAPI_PORT=36748\n", "SESSION_SECURE", "true")
	want := "SESSION_KEY=abc\nSESSION_SECURE=true\nAPI_PORT=36748\n"
	if got != want {
		t.Fatalf("update existing:\nwant %q\ngot  %q", want, got)
	}

	got = setDotEnvValue("SESSION_KEY=abc\n", "SESSION_SECURE", "true")
	want = "SESSION_KEY=abc\nSESSION_SECURE=true\n"
	if got != want {
		t.Fatalf("append missing:\nwant %q\ngot  %q", want, got)
	}

	got = setDotEnvValue("# SESSION_SECURE=false\n", "SESSION_SECURE", "true")
	want = "# SESSION_SECURE=false\nSESSION_SECURE=true\n"
	if got != want {
		t.Fatalf("ignore comments:\nwant %q\ngot  %q", want, got)
	}
}
