// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sync"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

type Principal struct {
	UserID             *string
	Username           string
	GroupIDs           []string
	IsAnonymous        bool
	IsLegacy           bool
	MustChangePassword bool
}

type Evaluator struct {
	db                 database.Database
	regexCache         sync.Map
	allowAnonymousRead bool
	log                *logger.Logger
}

func NewEvaluator(db database.Database, log *logger.Logger, allowAnonymousRead bool) *Evaluator {
	return &Evaluator{
		db:                 db,
		allowAnonymousRead: allowAnonymousRead,
		log:                log,
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, principal Principal, repoName, artifactPath, action string) (bool, error) {
	if principal.IsLegacy {
		return true, nil
	}

	if principal.IsAnonymous {
		if action != "read" || !e.allowAnonymousRead {
			return false, nil
		}

		target, err := e.db.GetPermissionTargetByName(ctx, "anonymous-read")
		if err != nil {
			return false, err
		}

		if target == nil {
			return false, nil
		}

		return e.matchTarget(target, repoName, artifactPath, action)
	}

	if principal.UserID == nil {
		return false, nil
	}

	targets, err := e.db.GetPermissionTargetsForPrincipal(ctx, *principal.UserID, principal.GroupIDs)
	if err != nil {
		return false, err
	}

	for _, target := range targets {
		ok, err := e.matchTarget(target, repoName, artifactPath, action)
		if err != nil {
			return false, err
		}

		if ok {
			return true, nil
		}
	}

	return false, nil
}

func (e *Evaluator) matchTarget(target *models.PermissionTarget, repoName, artifactPath, action string) (bool, error) {
	repoRe, err := e.getRegexp(target.RepositoryFilter)
	if err != nil {
		return false, fmt.Errorf("compile repo filter %q: %w", target.RepositoryFilter, err)
	}

	pathRe, err := e.getRegexp(target.PathPattern)
	if err != nil {
		return false, fmt.Errorf("compile path pattern %q: %w", target.PathPattern, err)
	}

	if !repoRe.MatchString(repoName) {
		return false, nil
	}

	if !pathRe.MatchString(artifactPath) {
		return false, nil
	}

	if slices.Contains(target.Actions, action) {
		return true, nil
	}

	return false, nil
}

func (e *Evaluator) getRegexp(pattern string) (*regexp.Regexp, error) {
	if pattern == "*" {
		pattern = ".*"
	}

	if v, ok := e.regexCache.Load(pattern); ok {
		if re, ok := v.(*regexp.Regexp); ok {
			return re, nil
		}
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	e.regexCache.Store(pattern, re)

	return re, nil
}
