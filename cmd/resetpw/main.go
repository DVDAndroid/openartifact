// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/openartifact/openartifact/internal/auth"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

func run() error {
	if len(os.Args) < 2 || os.Args[1] == "" {
		fmt.Fprintln(os.Stderr, "usage: resetpw <new-admin-password>")
		fmt.Fprintln(os.Stderr, "database connection is read from DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE")
		os.Exit(2)
	}

	pw := os.Args[1]

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"),
		envOr("DB_PASSWORD", ""),
		envOr("DB_NAME", "artifact_repo"),
		envOr("DB_SSLMODE", "disable"),
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}

	//nolint:noctx // CLI tool, no context available
	if _, err := db.Exec(
		"UPDATE auth_users SET password_hash=$1, must_change_password=false WHERE username='admin'",
		hash,
	); err != nil {
		return err
	}

	fmt.Println("admin password reset successfully")

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
