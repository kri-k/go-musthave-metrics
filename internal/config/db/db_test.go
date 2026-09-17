package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPostgresEmptyDSN(t *testing.T) {
	database, err := NewPostgres("")
	require.NoError(t, err)
	require.Nil(t, database)
}
