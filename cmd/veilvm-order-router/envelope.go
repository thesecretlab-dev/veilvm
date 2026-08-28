package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const envelopeMagic = "VEILENC1"

type windowKeys struct {
	mu    sync.Mutex
	keys  map[uint64][]byte
	path  string
}

func newWindowKeys(path string) *windowKeys {
	w := &windowKeys{keys: map[uint64][]byte{}, path: path}
	w.load()
	return w
}

func (w *windowKeys) load() {
	if w.path == "" {
		return
	}
	raw, err := os.ReadFile(w.path)
	if err != nil {
		return
	}
	var dumped map[string]string
	if err := json.Unmarshal(raw, &dumped); err != nil {
		return
	}
	for k, hexKey := range dumped {
		var windowID uint64
		if _, err := fmt.Sscanf(k, "%d", &windowID); err != nil {
			continue
		}
		b, err := hex.DecodeString(hexKey)
		if err != nil || len(b) != 32 {
			continue
		}
		w.keys[windowID] = b
	}
}

func (w *windowKeys) persist() {
	if w.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(w.path), 0o755)
	dumped := make(map[string]string, len(w.keys))
	for id, key := range w.keys {
		dumped[fmt.Sprintf("%d", id)] = hex.EncodeToString(key)
	}
	raw, err := json.MarshalIndent(dumped, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(w.path, raw, 0o600)
}

func (w *windowKeys) key(windowID uint64) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if k, ok := w.keys[windowID]; ok {
		return append([]byte(nil), k...), nil
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	w.keys[windowID] = k
	w.persist()
	return append([]byte(nil), k...), nil
}

func sealEnvelope(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("window key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(envelopeMagic)+len(nonce)+len(ct))
	out = append(out, envelopeMagic...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

func openEnvelope(key, envelope []byte) ([]byte, error) {
	if !isSealedEnvelope(envelope) {
		return nil, errors.New("envelope is not VEILENC1 ciphertext")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	body := envelope[len(envelopeMagic):]
	if len(body) < gcm.NonceSize() {
		return nil, errors.New("envelope too short")
	}
	nonce, ct := body[:gcm.NonceSize()], body[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

func isSealedEnvelope(env []byte) bool {
	return len(env) > len(envelopeMagic) && string(env[:len(envelopeMagic)]) == envelopeMagic
}

func envelopeLooksPlaintext(env []byte) bool {
	s := strings.ToLower(string(env))
	return strings.Contains(s, "veil-order-v1") || strings.Contains(s, "|yes|") || strings.Contains(s, "|no|")
}

func sha256Bytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
