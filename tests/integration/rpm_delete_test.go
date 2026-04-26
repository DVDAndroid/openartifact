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

func TestRPMDelete(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "delete-test-repo")
	rpmData := CreateTestRPM("delpkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	var uploadResp struct {
		Data struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}

	filename := "delpkg-1.0.0-1.el9.x86_64.rpm"
	comp, err := harness.Database.GetComponentByFilename(ctx, repo.ID, filename)
	if err != nil {
		t.Fatalf("GetComponentByFilename: %v", err)
	}
	if comp == nil {
		t.Fatal("expected component after upload")
	}

	deleteURL := fmt.Sprintf("/api/v1/repositories/%s/packages/%s", repo.Name, comp.ID)
	req := httptest.NewRequestWithContext(context.Background(), "DELETE", deleteURL, nil)
	dw := httptest.NewRecorder()
	router.ServeHTTP(dw, req)

	if dw.Code != http.StatusOK {
		t.Fatalf("DELETE: expected 200, got %d: %s", dw.Code, dw.Body.String())
	}

	servingRouter := buildServingRouter(t)
	pkgURL := fmt.Sprintf("/repos/%s/Packages/delpkg-1.0.0-1.el9.x86_64.rpm", repo.Name)
	getReq := httptest.NewRequestWithContext(context.Background(), "GET", pkgURL, nil)
	gw := httptest.NewRecorder()
	servingRouter.ServeHTTP(gw, getReq)

	if gw.Code != http.StatusNotFound {
		t.Errorf("GET after delete: expected 404, got %d", gw.Code)
	}
}

func TestRPMDelete_MetadataStale(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "delete-stale-repo")
	rpmData := CreateTestRPM("staledelpkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	filename := "staledelpkg-1.0.0-1.el9.x86_64.rpm"
	comp, err := harness.Database.GetComponentByFilename(ctx, repo.ID, filename)
	if err != nil {
		t.Fatalf("GetComponentByFilename: %v", err)
	}
	if comp == nil {
		t.Fatal("expected component after upload")
	}

	deleteURL := fmt.Sprintf("/api/v1/repositories/%s/packages/%s", repo.Name, comp.ID)
	req := httptest.NewRequestWithContext(context.Background(), "DELETE", deleteURL, nil)
	dw := httptest.NewRecorder()
	router.ServeHTTP(dw, req)

	if dw.Code != http.StatusOK {
		t.Fatalf("DELETE: expected 200, got %d: %s", dw.Code, dw.Body.String())
	}

	rpmRepo, err := harness.Database.GetRPMRepo(ctx, repo.ID)
	if err != nil {
		t.Fatalf("GetRPMRepo: %v", err)
	}
	if rpmRepo == nil {
		t.Fatal("expected rpm_repos row")
	}
	if !rpmRepo.MetadataStale {
		t.Error("expected metadata_stale = true after delete")
	}
}
