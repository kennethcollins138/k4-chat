package tokens

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRefreshTokenID(t *testing.T) {
	id1, err := GenerateRefreshTokenID()
	require.NoError(t, err, "GenerateRefreshTokenID failed on first call")
	assert.NotEmpty(t, id1, "Generated ID1 should not be empty")

	// Check decoded length
	decodedID1, err := base64.URLEncoding.DecodeString(id1)
	require.NoError(t, err, "Failed to decode ID1")
	assert.Len(t, decodedID1, 32, "Decoded ID1 should be 32 bytes")

	id2, err := GenerateRefreshTokenID()
	require.NoError(t, err, "GenerateRefreshTokenID failed on second call")
	assert.NotEmpty(t, id2, "Generated ID2 should not be empty")

	// Check decoded length for second ID
	decodedID2, err := base64.URLEncoding.DecodeString(id2)
	require.NoError(t, err, "Failed to decode ID2")
	assert.Len(t, decodedID2, 32, "Decoded ID2 should be 32 bytes")

	assert.NotEqual(t, id1, id2, "Two generated IDs should be different")
}
