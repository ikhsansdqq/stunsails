package auth

import (
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1, truncated to 6 digits.
func TestTOTPVectors(t *testing.T) {
	tp := &TOTP{key: []byte("12345678901234567890")}
	for sec, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		if got := tp.Code(time.Unix(sec, 0)); got != want {
			t.Errorf("at %d: got %s, want %s", sec, got, want)
		}
	}
}

func TestTOTPReplayAndDrift(t *testing.T) {
	tp, err := ParseSecret(NewSecret())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	prev := tp.Code(now.Add(-30 * time.Second))
	if !tp.Verify(prev, now) {
		t.Fatal("code from the previous step should pass")
	}
	if tp.Verify(prev, now) {
		t.Fatal("the same code must not pass twice")
	}
	if !tp.Verify(tp.Code(now), now) {
		t.Fatal("current code should pass")
	}
	if tp.Verify(tp.Code(now.Add(-60*time.Second)), now) {
		t.Fatal("a code two steps old must fail")
	}
	if tp.Verify("12345", now) || tp.Verify("abcdef", now) {
		t.Fatal("malformed codes must fail")
	}
}

func TestParseSecretTolerant(t *testing.T) {
	s := NewSecret()
	a, _ := ParseSecret(s)
	b, err := ParseSecret(" " + s[:8] + " " + s[8:] + "\n")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if a.Code(now) != b.Code(now) {
		t.Fatal("spaces and newlines should be ignored")
	}
	if _, err := ParseSecret("not base32!"); err == nil {
		t.Fatal("want error")
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter()
	now := time.Unix(1000, 0)
	for range 5 {
		if ok, _ := l.Allowed("1.2.3.4", now); !ok {
			t.Fatal("should be allowed before 5 failures")
		}
		l.Fail("1.2.3.4", now)
	}
	if ok, until := l.Allowed("1.2.3.4", now); ok || !until.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("want locked until +15m, got ok=%v until=%v", ok, until)
	}
	if ok, _ := l.Allowed("5.6.7.8", now); !ok {
		t.Fatal("other addresses stay allowed")
	}
	if ok, _ := l.Allowed("1.2.3.4", now.Add(15*time.Minute)); !ok {
		t.Fatal("lockout should end after the window")
	}
	l2 := NewLimiter()
	for i := range 50 {
		l2.Fail(string(rune('a'+i%26))+string(rune('a'+i/26)), now)
	}
	if ok, _ := l2.Allowed("fresh", now); !ok {
		t.Fatal("the global cap slows tries down but never locks a fresh address out")
	}
	if d := l2.Delay(now); d != 2*time.Second {
		t.Fatalf("over the global cap every try waits: %v", d)
	}
	if d := NewLimiter().Delay(now); d != 0 {
		t.Fatalf("under the cap, no wait: %v", d)
	}
	l4 := NewLimiter()
	for range 5 {
		l4.Fail("1.2.3.4", now)
	}
	if n := l4.Left("1.2.3.4", now); n != 0 {
		t.Fatalf("locked address has 0 tries left, got %d", n)
	}
	if n := l4.Left("9.9.9.9", now); n != 5 {
		t.Fatalf("fresh address has 5 tries left, got %d", n)
	}
	l3 := NewLimiter()
	l3.Restore(l4.Snapshot())
	if ok, _ := l3.Allowed("1.2.3.4", now); ok {
		t.Fatal("a lockout survives a restart")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct horse") || CheckPassword(h, "wrong") || CheckPassword("", "") {
		t.Fatal("password check is wrong")
	}
}
