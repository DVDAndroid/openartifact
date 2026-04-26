// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDedup_SamePackageTwice(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "dedup-same-repo")
	rpmData := CreateTestRPM("deduppkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w1 := uploadRPM(t, router, rpmData, repo.Name)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	w2 := uploadRPM(t, router, rpmData, repo.Name)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second upload: expected 409, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal 409 response: %v", err)
	}
	if resp["success"] == true {
		t.Error("expected success=false for duplicate")
	}
	if resp["code"] != "CONFLICT" {
		t.Errorf("expected code=CONFLICT, got %v", resp["code"])
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object in response, got %T", resp["data"])
	}
	if data["duplicate"] != true {
		t.Errorf("expected duplicate=true, got %v", data["duplicate"])
	}
}

func TestDedup_CrossRepo(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repoA := createTestRepo(t, ctx, "dedup-cross-repo-a")
	repoB := createTestRepo(t, ctx, "dedup-cross-repo-b")
	rpmData := CreateTestRPM("crossdeduppkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	wA := uploadRPM(t, router, rpmData, repoA.Name)
	if wA.Code != http.StatusOK {
		t.Fatalf("repo-a upload: expected 200, got %d: %s", wA.Code, wA.Body.String())
	}

	wB := uploadRPM(t, router, rpmData, repoB.Name)
	if wB.Code != http.StatusOK {
		t.Fatalf("repo-b upload: expected 200 (cross-repo dedup allows), got %d: %s", wB.Code, wB.Body.String())
	}
}

func TestDedup_DeleteThenReupload(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "dedup-del-reup-repo")
	rpmData := CreateTestRPM("delreuppkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w1 := uploadRPM(t, router, rpmData, repo.Name)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	filename := "delreuppkg-1.0.0-1.el9.x86_64.rpm"
	comp, err := harness.Database.GetComponentByFilename(ctx, repo.ID, filename)
	if err != nil {
		t.Fatalf("GetComponentByFilename: %v", err)
	}
	if comp == nil {
		t.Fatal("expected component after first upload")
	}

	deleteURL := fmt.Sprintf("/api/v1/repositories/%s/packages/%s", repo.Name, comp.ID)
	deleteReq := httptest.NewRequestWithContext(context.Background(), "DELETE", deleteURL, nil)
	dw := httptest.NewRecorder()
	router.ServeHTTP(dw, deleteReq)
	if dw.Code != http.StatusOK {
		t.Fatalf("DELETE: expected 200, got %d: %s", dw.Code, dw.Body.String())
	}

	w2 := uploadRPM(t, router, rpmData, repo.Name)
	if w2.Code != http.StatusOK {
		t.Fatalf("re-upload after delete: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}
