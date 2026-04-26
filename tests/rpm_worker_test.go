// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	formatrpm "github.com/openartifact/openartifact/internal/format/rpm"
)

func TestDebounceNotifyStale(t *testing.T) {
	regenerations := atomic.Int64{}
	debounce := 100 * time.Millisecond

	var mu sync.Mutex

	timers := make(map[string]*time.Timer)

	notify := func(repoID string) {
		mu.Lock()
		if t, ok := timers[repoID]; ok {
			t.Stop()
		}

		timers[repoID] = time.AfterFunc(debounce, func() {
			regenerations.Add(1)
			mu.Lock()
			delete(timers, repoID)
			mu.Unlock()
		})
		mu.Unlock()
	}

	notify("repo-1")
	notify("repo-1")
	notify("repo-1")

	time.Sleep(debounce + 50*time.Millisecond)

	if regenerations.Load() != 1 {
		t.Errorf("expected 1 regeneration after debounce, got %d", regenerations.Load())
	}
}

func TestDebounceDifferentRepos(t *testing.T) {
	debounce := 100 * time.Millisecond
	regenerations := map[string]*atomic.Int64{
		"repo-a": {},
		"repo-b": {},
	}

	var mu sync.Mutex

	timers := make(map[string]*time.Timer)

	notify := func(repoID string) {
		mu.Lock()
		if t, ok := timers[repoID]; ok {
			t.Stop()
		}

		timers[repoID] = time.AfterFunc(debounce, func() {
			regenerations[repoID].Add(1)
			mu.Lock()
			delete(timers, repoID)
			mu.Unlock()
		})
		mu.Unlock()
	}

	notify("repo-a")
	notify("repo-b")

	time.Sleep(debounce + 50*time.Millisecond)

	if regenerations["repo-a"].Load() != 1 {
		t.Errorf("repo-a regenerations = %d, want 1", regenerations["repo-a"].Load())
	}

	if regenerations["repo-b"].Load() != 1 {
		t.Errorf("repo-b regenerations = %d, want 1", regenerations["repo-b"].Load())
	}
}

func TestDebounceReset(t *testing.T) {
	regenerations := atomic.Int64{}
	debounce := 200 * time.Millisecond

	var (
		mu    sync.Mutex
		timer *time.Timer
	)

	notify := func() {
		mu.Lock()
		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(debounce, func() {
			regenerations.Add(1)
			mu.Lock()
			timer = nil
			mu.Unlock()
		})
		mu.Unlock()
	}

	notify()
	time.Sleep(100 * time.Millisecond)
	notify()
	time.Sleep(100 * time.Millisecond)
	notify()

	if regenerations.Load() != 0 {
		t.Errorf("expected 0 regenerations before debounce, got %d", regenerations.Load())
	}

	time.Sleep(debounce + 50*time.Millisecond)

	if regenerations.Load() != 1 {
		t.Errorf("expected 1 regeneration after final debounce, got %d", regenerations.Load())
	}
}

func TestCompressEmptyData(t *testing.T) {
	_, err := formatrpm.CompressMetadata(nil)
	if err == nil {
		t.Error("CompressMetadata(nil) expected error, got nil")
	}

	_, err = formatrpm.CompressMetadata([]byte{})
	if err == nil {
		t.Error("CompressMetadata(empty) expected error, got nil")
	}
}

func TestSignerNilSigner(t *testing.T) {
	_, err := formatrpm.SignRepomdXML(nil, []byte("<repomd/>"))
	if err == nil {
		t.Error("SignRepomdXML(nil signer) expected error, got nil")
	}

	_, err = formatrpm.PublicKey(nil)
	if err == nil {
		t.Error("PublicKey(nil signer) expected error, got nil")
	}

	_, err = formatrpm.KeyFingerprint(nil)
	if err == nil {
		t.Error("KeyFingerprint(nil signer) expected error, got nil")
	}

	_, err = formatrpm.CleartextSign(nil, []byte("data"))
	if err == nil {
		t.Error("CleartextSign(nil signer) expected error, got nil")
	}
}

func TestSignerEmptyData(t *testing.T) {
	mockSigner := &nilSigner{}

	_, err := formatrpm.SignRepomdXML(mockSigner, nil)
	if err == nil {
		t.Error("SignRepomdXML(empty data) expected error, got nil")
	}

	_, err = formatrpm.SignRepomdXML(mockSigner, []byte{})
	if err == nil {
		t.Error("SignRepomdXML(empty data) expected error, got nil")
	}
}

type nilSigner struct{}

func (n *nilSigner) CleartextSign(data io.Reader) (io.Reader, error) { return nil, nil }
func (n *nilSigner) DetachedSign(data io.Reader) ([]byte, error)     { return nil, nil }
func (n *nilSigner) PublicKey() ([]byte, error)                      { return nil, nil }
func (n *nilSigner) KeyFingerprint() string                          { return "" }
