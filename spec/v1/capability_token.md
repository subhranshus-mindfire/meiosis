# CapabilityToken (v1)

A **CapabilityToken** grants a Principal a narrow, time-bounded permission to
operate within an Intent's declared scope. It is the credential a Principal
presents before a write is admitted (FR-1.3/FR-1.4); `Intent.scope` is still
enforced separately at commit time (FR-3.4).

The token is signed by the issuing Principal, not the holder: `principal` is
who the capability is granted to, `issued_by` is who granted it and whose key
produced `signature`. A Principal can therefore never mint or widen its own
authority — it can only be delegated a subset of what an issuer already holds
(FR-1.5).

## Object Definition

### Core Fields
- `id` (String, required): `CapabilityTokenID` in the format `"cap_{base32(32 random bytes)}"`. Unlike `IntentID`/`AttemptID`, this ID is **random, not content-derived** — two tokens issued with otherwise identical fields still need independent identities so that one can be revoked without affecting the other.
- `principal` (String, required): `PrincipalID` the capability is granted to (the holder).
- `issued_by` (String, required): `PrincipalID` granting the capability (the issuer). Signs the token.
- `intent` (String, required): `IntentID` this capability is scoped to.
- `scope` (Object, required): Path glob constraints, same shape as `Intent.scope` (`allow`, `deny`, `mode`). `deny` overrides `allow`.
- `issued_at` (Timestamp, required): ISO8601 timestamp.
- `expires_at` (Timestamp, required): ISO8601 timestamp. Must be strictly after `issued_at`.
- `signature` (Sig, required): Ed25519 signature by `issued_by`.

## Validation Rules & Relationships

1. **Issuer-Signs, Not Holder (FR-1.5):** The signature is produced by `issued_by`'s key, never `principal`'s. This is what makes self-escalation structurally impossible — a Principal cannot mint or widen its own authority, only receive a delegated subset of what an issuer already holds.
2. **Expiry Ordering:** `expires_at` must be strictly after `issued_at`; a token with `expires_at <= issued_at` is invalid.
3. **Short-Lived by Default (NFR-4.3):** Implementations should default new tokens to a short TTL (the reference implementation defaults to one hour).
4. **Revocation (FR-1.7):** A token's `id` may be revoked independently of any other token, including ones issued to the same principal with identical fields — this is the reason `id` is random rather than content-derived. Revoking a Principal revokes all capabilities delegated from it, transitively.
5. **Scope Is Independent of Intent.scope:** A capability token's own `scope` is evaluated at authorization time (deny-first, then allow, default-deny) against the *token's* grant. This is a distinct check from the Intent's own declared scope enforcement (FR-3.4) — a real write path evaluates both.
