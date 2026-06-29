package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

func generateCacheKey(tenantID uint64, kbIDs []string) string {
	sort.Strings(kbIDs)
	data := strings.Join(kbIDs, ",")
	hash := sha256.Sum256([]byte(data))
	hashStr := hex.EncodeToString(hash[:])[:16]
	return hashStr
}

func generateEntryID() string {
	hash := sha256.Sum256([]byte(time.Now().String() + string(rune(time.Now().UnixNano()))))
	return hex.EncodeToString(hash[:])[:24]
}
