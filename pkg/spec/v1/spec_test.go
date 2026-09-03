package v1

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
)

func TestWorldHashJSON(t *testing.T) {
	h, err := ParseWorldHash("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("ParseWorldHash() error = %v", err)
	}
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(b), `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`; got != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestVerdictValidate(t *testing.T) {
	for _, decision := range []VerdictDecision{VerdictDecisionMerge, VerdictDecisionReject, VerdictDecisionEscalate} {
		tt := Verdict{Attempt: validAttemptID(), Decision: decision, DecidedBy: "human:lakin", PolicyRef: "policy @ hash", PolicyIn: json.RawMessage(`{"allow":true}`), Rationale: "reviewed", DecidedAt: time.Unix(1, 0), Signature: "sig"}
		if err := tt.Validate(); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
	}
	if err := (Verdict{Attempt: validAttemptID(), Decision: "bad", DecidedBy: "human:lakin", PolicyRef: "p", PolicyIn: json.RawMessage(`{}`), Rationale: "r", DecidedAt: time.Unix(1, 0), Signature: "s"}).Validate(); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestPrincipalValidate(t *testing.T) {
	if err := (Principal{ID: "agent:planner-7", Kind: PrincipalKindAgent}).Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if err := (Principal{Kind: PrincipalKindAgent}).Validate(); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestAttemptValidateAndJSON(t *testing.T) {
	h := MustParseWorldHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	a := Attempt{ID: validAttemptID(), Intent: validIntentID(), Author: "human:lakin", World: h, BaseWorld: h, Status: AttemptStatusOpen, CreatedAt: time.Unix(1, 0), Signature: "sig"}
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if len(b) == 0 {
		t.Fatal("Marshal() produced empty output")
	}
}

func TestEvidenceValidate(t *testing.T) {
	h := MustParseWorldHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	e := Evidence{ID: "e1", Attempt: validAttemptID(), Kind: EvidenceKindTestRun, Producer: "human:lakin", World: h, Outcome: EvidenceOutcomePass, Payload: json.RawMessage(`{}`), CreatedAt: time.Unix(2, 0), Signature: "sig"}
	if err := e.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestWorldHashRejectsInvalidJSON(t *testing.T) {
	var h WorldHash
	if err := json.Unmarshal([]byte(`"not-a-hash"`), &h); err == nil {
		t.Fatal("Unmarshal() expected error")
	}
}

func TestIntentValidate(t *testing.T) {
	i := Intent{
		ID: validIntentID(), Repo: "github.com/example/repo", Title: "Add feature", Goal: "Implement feature",
		Acceptance: []Criterion{{Text: "tests pass"}}, Scope: Scope{Allow: []string{"pkg/**"}, Mode: ScopeModeEnforce},
		CreatedBy: "human:lakin", CreatedAt: time.Unix(1, 0), Status: IntentStatusOpen, Signature: "sig",
	}
	if err := i.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	i.Scope.Allow = []string{"["}
	if err := i.Validate(); err == nil {
		t.Fatal("Validate() expected invalid glob error")
	}
}

func TestGenerateAndLoadKeyPair(t *testing.T) {
	original, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	if len(original.PublicKey) != ed25519.PublicKeySize || len(original.PrivateKey) != ed25519.PrivateKeySize {
		t.Fatal("GenerateKeyPair() returned incorrectly sized keys")
	}

	loaded, err := LoadKeyPair(original.PrivateKey, original.PublicKey)
	if err != nil {
		t.Fatalf("LoadKeyPair() error = %v", err)
	}
	if loaded.PublicKeyBase64() != original.PublicKeyBase64() || loaded.PrivateKeyBase64() != original.PrivateKeyBase64() {
		t.Fatal("LoadKeyPair() changed key material")
	}

	message := []byte("meiosis keypair test")
	signature := ed25519.Sign(loaded.PrivateKey, message)
	if !ed25519.Verify(loaded.PublicKey, message, signature) {
		t.Fatal("loaded keypair failed signature verification")
	}
}

func TestLoadEncodedKeyPair(t *testing.T) {
	original, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	loaded, err := LoadEncodedKeyPair(original.PrivateKeyBase64(), original.PublicKeyBase64())
	if err != nil {
		t.Fatalf("LoadEncodedKeyPair() error = %v", err)
	}
	if loaded.PublicKeyBase64() != original.PublicKeyBase64() {
		t.Fatal("LoadEncodedKeyPair() loaded a different public key")
	}
}

func TestLoadKeyPairRejectsMalformedKeys(t *testing.T) {
	valid, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	for name, privateKey := range map[string][]byte{
		"empty":     nil,
		"short":     make([]byte, ed25519.SeedSize-1),
		"malformed": make([]byte, ed25519.PrivateKeySize),
		"long":      make([]byte, ed25519.PrivateKeySize+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadKeyPair(privateKey, nil); err == nil {
				t.Fatal("LoadKeyPair() accepted malformed private key")
			}
		})
	}
	badPublic := make([]byte, ed25519.PublicKeySize)
	if _, err := LoadKeyPair(valid.PrivateKey, badPublic); err == nil {
		t.Fatal("LoadKeyPair() accepted mismatched public key")
	}
	if _, err := LoadEncodedKeyPair("not-base64", ""); err == nil {
		t.Fatal("LoadEncodedKeyPair() accepted malformed encoding")
	}
}

func validIntentID() string  { return "int_" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
func validAttemptID() string { return "att_" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" }
