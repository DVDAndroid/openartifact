// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/middleware"
)

const maxRequestBodyBytes int64 = 1 << 20

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(r *http.Request, dst any) error {
	limited := io.LimitReader(r.Body, maxRequestBodyBytes)
	return json.NewDecoder(limited).Decode(dst)
}

func currentPrincipal(r *http.Request) *auth.Principal {
	return middleware.PrincipalFromContext(r.Context())
}

func principalIsAdmin(ctx context.Context, p *auth.Principal, evaluator *auth.Evaluator) (bool, error) {
	if p == nil || p.IsAnonymous {
		return false, nil
	}

	return evaluator.Evaluate(ctx, *p, "", "", "manage")
}
