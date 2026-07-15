package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	encPrefix = "ENC["
	encSuffix = "]"
	// Version tags the on-disk token format so the scheme can evolve without
	// breaking repositories encrypted under an earlier version.
	Version   = "v1"
	Version2  = "v2"
	nonceSize = 12
	tagSize   = 16
)

// Encrypt deterministically encrypts a value and returns an ENC[v1:...] token.
//
// Determinism is achieved with a synthetic IV: the nonce is derived from the
// plaintext via HMAC, so identical plaintext yields identical ciphertext (which
// keeps Git diffs and merges clean) while distinct plaintext yields distinct
// nonces. Nonce reuse therefore only ever happens for identical plaintext,
// where reproducing the same ciphertext is exactly the desired behaviour.
func (k *Key) Encrypt(plaintext []byte) (string, error) {
	nonce := k.nonce(plaintext)
	sealed := k.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	token := base64.StdEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s%s:%s%s", encPrefix, Version, token, encSuffix), nil
}

// Decrypt reverses Encrypt, returning the original plaintext for an ENC token.
func (k *Key) Decrypt(token string) ([]byte, error) {
	inner, ok := unwrap(token)
	if !ok {
		return nil, fmt.Errorf("envapor: malformed encrypted token")
	}
	version, b64, found := strings.Cut(inner, ":")
	if !found || version != Version {
		return nil, fmt.Errorf("envapor: unsupported token version %q", version)
	}
	payload, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("envapor: decoding token: %w", err)
	}
	if len(payload) < nonceSize+tagSize {
		return nil, fmt.Errorf("envapor: token too short")
	}
	nonce, sealed := payload[:nonceSize], payload[nonceSize:]
	plaintext, err := k.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("envapor: authentication failed (wrong key or corrupt data): %w", err)
	}
	return plaintext, nil
}

// EncryptContext encrypts a value while binding it to a stable logical
// context, such as an environment-variable name. Version 2 tokens cannot be
// moved to a different context without authentication failing.
func (k *Key) EncryptContext(plaintext []byte, context string) (string, error) {
	aad := []byte(context)
	nonce := k.contextNonce(plaintext, aad)
	sealed := k.aead.Seal(nil, nonce, plaintext, aad)
	payload := append(nonce, sealed...)
	token := base64.StdEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s%s:%s%s", encPrefix, Version2, token, encSuffix), nil
}

// DecryptContext decrypts a context-bound v2 token. Legacy v1 tokens remain
// readable so repositories can migrate naturally as values change.
func (k *Key) DecryptContext(token, context string) ([]byte, error) {
	inner, ok := unwrap(token)
	if !ok {
		return nil, fmt.Errorf("envapor: malformed encrypted token")
	}
	version, b64, found := strings.Cut(inner, ":")
	if !found {
		return nil, fmt.Errorf("envapor: malformed encrypted token")
	}
	if version == Version {
		return k.Decrypt(token)
	}
	if version != Version2 {
		return nil, fmt.Errorf("envapor: unsupported token version %q", version)
	}
	payload, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("envapor: decoding token: %w", err)
	}
	if len(payload) < nonceSize+tagSize {
		return nil, fmt.Errorf("envapor: token too short")
	}
	aad := []byte(context)
	plaintext, err := k.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], aad)
	if err != nil {
		return nil, fmt.Errorf("envapor: authentication failed (wrong key, context, or corrupt data): %w", err)
	}
	return plaintext, nil
}

func (k *Key) nonce(plaintext []byte) []byte {
	mac := hmac.New(sha256.New, k.mac)
	mac.Write(plaintext)
	return mac.Sum(nil)[:nonceSize]
}

func (k *Key) contextNonce(plaintext, context []byte) []byte {
	mac := hmac.New(sha256.New, k.mac)
	mac.Write([]byte("envapor:v2\x00"))
	mac.Write(context)
	mac.Write([]byte{0})
	mac.Write(plaintext)
	return mac.Sum(nil)[:nonceSize]
}

// IsEncrypted reports whether s is a well-formed Envapor token of the current
// version.
func IsEncrypted(s string) bool {
	inner, ok := unwrap(s)
	if !ok {
		return false
	}
	version, b64, found := strings.Cut(inner, ":")
	if !found || (version != Version && version != Version2) || b64 == "" {
		return false
	}
	payload, err := base64.StdEncoding.DecodeString(b64)
	return err == nil && len(payload) >= nonceSize+tagSize
}

func unwrap(s string) (string, bool) {
	if !strings.HasPrefix(s, encPrefix) || !strings.HasSuffix(s, encSuffix) {
		return "", false
	}
	return s[len(encPrefix) : len(s)-len(encSuffix)], true
}
