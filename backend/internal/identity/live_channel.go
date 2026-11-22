package identity

import (
	"crypto/sha256"
	"encoding/hex"
)

func OrganizationChannelKey(organizationID string) string {
	digest := sha256.Sum256([]byte(organizationID))
	return hex.EncodeToString(digest[:16])
}
