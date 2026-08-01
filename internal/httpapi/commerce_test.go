package httpapi

import (
	"crypto/sha256"
	"strings"
	"testing"
)

func TestRedeemCodesTextIsOneCodePerLine(t *testing.T) {
	content := redeemCodesText([]string{"DEER-AAAA-BBBB-CCCC-DDDD", "DEER-1111-2222-3333-4444"})
	if content != "DEER-AAAA-BBBB-CCCC-DDDD\nDEER-1111-2222-3333-4444\n" {
		t.Fatalf("content=%q", content)
	}
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if strings.Contains(line, ",") || strings.Contains(line, "batch_id") {
			t.Fatalf("unexpected non-code content: %q", line)
		}
	}
}

func TestRedeemCodeSearchUsesHashForCompleteCode(t *testing.T) {
	hash, query := redeemCodeSearch("deer-aaaa-bbbb-cccc-dddd")
	if len(hash) != sha256.Size || query != "" {
		t.Fatalf("hash search=%x query=%q", hash, query)
	}
	if _, query := redeemCodeSearch("viewer@example.com"); query != "viewer@example.com" {
		t.Fatalf("email search=%q", query)
	}
}
