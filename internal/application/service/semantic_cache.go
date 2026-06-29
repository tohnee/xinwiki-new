package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

func generateCacheKey(tenantID uint64, kbIDs []string) string {
	sort.Strings(kbIDs)
	// Include tenantID in the hash to guarantee tenant isolation — without this,
	// two tenants with the same KB IDs would collide and leak cached results
	// across tenants (a cross-tenant information disclosure vulnerability).
	data := fmt.Sprintf("%d|%s", tenantID, strings.Join(kbIDs, ","))
	hash := sha256.Sum256([]byte(data))
	hashStr := hex.EncodeToString(hash[:])[:16]
	return hashStr
}

func generateEntryID() string {
	hash := sha256.Sum256([]byte(time.Now().String() + string(rune(time.Now().UnixNano()))))
	return hex.EncodeToString(hash[:])[:24]
}
