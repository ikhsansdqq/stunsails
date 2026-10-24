// Package auth holds StunSails' sign-in pieces: the password hash, the
// authenticator (TOTP) secret, and the per-address limit on failed tries.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	step   = 30 * time.Second
	digits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// TOTP checks RFC 6238 codes (SHA-1, 6 digits, 30 s), as Google Authenticator makes them.
// Each time step is accepted once, so a code seen over someone's shoulder can't be replayed.
type TOTP struct {
	key []byte

	mu   sync.Mutex
	last int64 // highest step accepted so far
}

func ParseSecret(s string) (*TOTP, error) {
	s = strings.ToUpper(strings.Join(strings.Fields(s), ""))
	s = strings.TrimRight(s, "=")
	key, err := b32.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("authenticator secret is not base32: %w", err)
	}
	if len(key) < 10 {
		return nil, errors.New("authenticator secret is too short")
	}
	return &TOTP{key: key}, nil
}

// NewSecret returns a fresh 160-bit base32 secret.
func NewSecret() string {
	b := make([]byte, 20)
	rand.Read(b)
	return b32.EncodeToString(b)
}

// URI is the otpauth link the authenticator app scans.
func URI(secret, issuer, account string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}

func (t *TOTP) code(counter int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	h := hmac.New(sha1.New, t.key)
	h.Write(msg[:])
	sum := h.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", digits, n%1_000_000)
}

// Code is the current code, for tests and the enroll check.
func (t *TOTP) Code(now time.Time) string { return t.code(now.Unix() / int64(step.Seconds())) }

// Verify accepts the code for now or one step either side (clock drift), once.
func (t *TOTP) Verify(code string, now time.Time) bool {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != digits {
		return false
	}
	cur := now.Unix() / int64(step.Seconds())
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range []int64{cur, cur - 1, cur + 1} {
		if c <= t.last {
			continue
		}
		if hmac.Equal([]byte(t.code(c)), []byte(code)) {
			t.last = c
			return true
		}
	}
	return false
}
