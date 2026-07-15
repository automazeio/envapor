package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	// PEMType is the PEM block type used for Envapor key files.
	PEMType = "ENVAPOR KEY"
	// keySize is the length in bytes of the master key and derived subkeys.
	keySize = 32
)

// Key holds an Envapor master key along with the subkeys derived from it.
// The master key is symmetric and shared across a team: every holder can both
// encrypt and decrypt, which is what makes deterministic, mergeable ciphertext
// possible.
type Key struct {
	master []byte
	mac    []byte
	aead   cipher.AEAD
}

// Generate creates a new random master key with safe parameters.
func Generate() (*Key, error) {
	master := make([]byte, keySize)
	if _, err := rand.Read(master); err != nil {
		return nil, fmt.Errorf("envapor: generating key: %w", err)
	}
	return derive(master)
}

// derive expands a master key into independent encryption and MAC subkeys via
// HKDF so the raw master is never used directly by either primitive.
func derive(master []byte) (*Key, error) {
	if len(master) != keySize {
		return nil, fmt.Errorf("envapor: invalid key length %d, want %d", len(master), keySize)
	}
	enc, err := hkdf.Key(sha256.New, master, nil, "envapor:v1:enc", keySize)
	if err != nil {
		return nil, fmt.Errorf("envapor: deriving encryption key: %w", err)
	}
	mac, err := hkdf.Key(sha256.New, master, nil, "envapor:v1:mac", keySize)
	if err != nil {
		return nil, fmt.Errorf("envapor: deriving mac key: %w", err)
	}
	block, err := aes.NewCipher(enc)
	if err != nil {
		return nil, fmt.Errorf("envapor: initializing cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("envapor: initializing GCM: %w", err)
	}
	return &Key{master: master, mac: mac, aead: aead}, nil
}

// MarshalPEM encodes the master key as a PEM block suitable for writing to disk.
func (k *Key) MarshalPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: PEMType, Bytes: k.master})
}

// Destroy zeroes the in-memory key material. It is best-effort defense in
// depth for a short-lived CLI: the AEAD retains an internal copy of the
// expanded encryption key that Go does not expose for clearing, so this only
// scrubs the master and MAC secrets we hold directly. It is safe to call on a
// nil Key and after the key is no longer needed (MarshalPEM will not work
// afterwards, since the master bytes are cleared).
func (k *Key) Destroy() {
	if k == nil {
		return
	}
	clear(k.master)
	clear(k.mac)
}

// LoadPEM parses a PEM-encoded Envapor key file.
func LoadPEM(data []byte) (*Key, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("envapor: no PEM block found in key file")
	}
	if block.Type != PEMType {
		return nil, fmt.Errorf("envapor: unexpected PEM type %q, want %q", block.Type, PEMType)
	}
	return derive(block.Bytes)
}
