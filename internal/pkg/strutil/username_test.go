package strutil

import (
	"strings"
	"testing"
)

func TestEncodeUserIDUsername_FormatAndUnique(t *testing.T) {
	seen := make(map[string]int, 20000)
	for id := 1; id <= 20000; id++ {
		name := EncodeUserIDUsername(id)
		if !strings.HasPrefix(name, UsernamePrefix) {
			t.Fatalf("id=%d username=%q missing prefix", id, name)
		}
		if len(name) != len(UsernamePrefix)+usernameBodyLen {
			t.Fatalf("id=%d username=%q unexpected length", id, name)
		}
		if !IsAutoUsername(name) {
			t.Fatalf("id=%d username=%q invalid charset", id, name)
		}
		if prev, ok := seen[name]; ok {
			t.Fatalf("collision: id=%d and id=%d -> %q", prev, id, name)
		}
		seen[name] = id
	}
}

func TestEncodeUserIDUsername_Deterministic(t *testing.T) {
	a := EncodeUserIDUsername(9527)
	b := EncodeUserIDUsername(9527)
	if a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}
	if EncodeUserIDUsernameWithSalt(9527, 1) == a {
		t.Fatalf("salt should change username appearance")
	}
}

func TestEncodeUserIDUsername_SaltKeepsIDUniqueness(t *testing.T) {
	// 不同 id 即使 salt 相同也不应冲突；同 id 不同 salt 也不应冲突
	seen := map[string]struct{}{}
	for id := 1; id <= 500; id++ {
		for salt := 0; salt < 8; salt++ {
			name := EncodeUserIDUsernameWithSalt(id, salt)
			if _, ok := seen[name]; ok {
				t.Fatalf("collision id=%d salt=%d name=%q", id, salt, name)
			}
			seen[name] = struct{}{}
		}
	}
}
