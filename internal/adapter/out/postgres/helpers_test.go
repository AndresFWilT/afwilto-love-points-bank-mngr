//go:build integration

package postgres_test

import "github.com/google/uuid"

func randUUID() string { return uuid.NewString() }
