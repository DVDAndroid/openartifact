// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package android

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/apk/apktest"
	"github.com/openartifact/openartifact/pkg/logger"
)

const systemOrgID = "00000000-0000-0000-0000-000000000001"

// env is a handler wired to a scratch Postgres database and a filesystem blob store.
type env struct {
	t      *testing.T
	db     *database.PostgresDB
	blobs  storage.BlobStore
	router chi.Router
}

func newEnv(t *testing.T) *env {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL not set in CI; integration services are required")
		}

		t.Skip("TEST_DATABASE_URL not set")
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = admin.Close() })

	dbName := fmt.Sprintf("oa_android_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(context.Background(), `CREATE DATABASE `+dbName); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS `+dbName+` WITH (FORCE)`)
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}

	port, _ := strconv.Atoi(u.Port())
	pw, _ := u.User.Password()

	log, err := logger.New("error", "json", "stderr")
	if err != nil {
		t.Fatal(err)
	}

	pg, err := database.NewPostgresDB(database.DatabaseConfig{
		Host: u.Hostname(), Port: port, User: u.User.Username(), Password: pw, Database: dbName,
		SSLMode: "disable", MaxOpenConns: 4, MaxIdleConns: 4,
	}, log)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = pg.Close() })

	if err := pg.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	blobs, err := storage.NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(log, pg, pg, blobs, blobs, "http://oa.test", nil)
	r := chi.NewRouter()
	h.Register(r)
	r.Route("/repos/{name}", h.RegisterRepoRoutes)

	return &env{t: t, db: pg, blobs: blobs, router: r}
}

func (e *env) repo(name, format string) {
	e.t.Helper()

	org := systemOrgID
	if err := e.db.CreateRepository(context.Background(), &models.Repository{Name: name, Format: format, RepoType: "hosted", OrgID: &org}); err != nil {
		e.t.Fatal(err)
	}
}

type reply struct {
	*httptest.ResponseRecorder
}

func (e *env) do(method, target string, body io.Reader, contentType string) reply {
	e.t.Helper()

	req := httptest.NewRequestWithContext(e.t.Context(), method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return reply{rec}
}

func (e *env) getJSON(target string) reply { return e.do(http.MethodGet, target, nil, "") }

func (e *env) putJSON(target string, v any) reply {
	b, _ := json.Marshal(v)

	return e.do(http.MethodPut, target, bytes.NewReader(b), "application/json")
}

func (e *env) postJSON(target string, v any) reply {
	b, _ := json.Marshal(v)

	return e.do(http.MethodPost, target, bytes.NewReader(b), "application/json")
}

// data decodes the "data" member of an enveloped answer.
func (r reply) data(t *testing.T, want int) map[string]any {
	t.Helper()

	if r.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", r.Code, want, r.Body.String())
	}

	var env struct {
		Data map[string]any `json:"data"`
	}

	if err := json.Unmarshal(r.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %q: %v", r.Body.String(), err)
	}

	return env.Data
}

func (r reply) wantStatus(t *testing.T, want int) {
	t.Helper()

	if r.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", r.Code, want, r.Body.String())
	}
}

type part struct {
	field, filename string
	content         []byte
}

func multipartBody(t *testing.T, fields map[string]string, parts ...part) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}

	for _, p := range parts {
		fw, err := mw.CreateFormFile(p.field, p.filename)
		if err != nil {
			t.Fatal(err)
		}

		_, _ = fw.Write(p.content)
	}

	_ = mw.Close()

	return &buf, mw.FormDataContentType()
}

func (e *env) upload(repo string, fields map[string]string, parts ...part) reply {
	e.t.Helper()

	body, ct := multipartBody(e.t, fields, parts...)

	return e.do(http.MethodPost, "/api/v1/repositories/"+repo+"/apks", body, ct)
}

func demoAPK(code int64, key *apktest.Key) []byte {
	b := apktest.Builder{
		Package: "org.example.app", VersionCode: code, VersionName: fmt.Sprintf("1.%d", code), MinSDK: 21, TargetSDK: 34,
		Labels: map[string]string{"": "Demo", "it": "Dimostrazione"}, Icons: map[uint16]int{480: 96, 320: 72},
	}

	return b.Build(apktest.Sign{Key: key, V1: true, V2: true})
}

func TestAndroidUploadOverQuota(t *testing.T) {
	e := newEnv(t)
	e.repo("tiny", "android")

	repo, err := e.db.GetRepositoryByName(context.Background(), "tiny")
	if err != nil || repo == nil {
		t.Fatalf("repository: %v %v", repo, err)
	}

	q, err := e.db.GetOrCreateStorageQuota(context.Background(), repo.ID)
	if err != nil {
		t.Fatal(err)
	}

	q.MaxSizeBytes = 1024
	if err := e.db.UpdateStorageQuota(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	apkBytes := demoAPK(1, apktest.NewRSAKey("Me"))
	e.upload("tiny", nil, part{"file", "a.apk", apkBytes}).wantStatus(t, http.StatusInsufficientStorage)

	if apps := e.getJSON("/api/v1/repositories/tiny/apks/apps").data(t, http.StatusOK); apps["total"].(float64) != 0 {
		t.Fatalf("a refused upload left an app behind: %v", apps)
	}

	sum := sha256.Sum256(apkBytes)
	if rc, err := e.blobs.Get(context.Background(), hex.EncodeToString(sum[:])); err == nil {
		_ = rc.Close()

		t.Error("a refused upload left its file in the blob store")
	}
}
