package im

import (
	"context"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

const (
	dedupTTL                 = 5 * time.Minute
	dedupCleanupInterval     = 1 * time.Minute
	maxContentLength         = 4096
	maxQuoteContentLength    = 500
	streamFlushInterval      = 300 * time.Millisecond
	agentCompleteWaitTimeout = 10 * time.Second
)

var imCitationTagRe = regexp.MustCompile(`<(?:kb|web)\b[^>]*/?>`)

func stripIMCitationTags(s string) string {
	return imCitationTagRe.ReplaceAllString(s, "")
}

var imageXMLBlockRe = regexp.MustCompile(`(?s)<image\b[^>]*>.*?</image>`)

var imageOriginalRe = regexp.MustCompile(`<image_original>(.*?)</image_original>`)

func stripImageXMLTags(s string) string {
	return imageXMLBlockRe.ReplaceAllStringFunc(s, func(block string) string {
		if m := imageOriginalRe.FindStringSubmatch(block); len(m) > 1 {
			return m[1]
		}
		return ""
	})
}

var storageSchemeRe = regexp.MustCompile(`\b(local|minio|s3|cos|tos|oss)://[^\s)\]>"]+`)

func rewriteStorageURLs(ctx context.Context, content string, resolver *imFileServiceResolver) string {
	if resolver == nil {
		return content
	}
	return storageSchemeRe.ReplaceAllStringFunc(content, func(match string) string {
		fileSvc := resolver.resolve(match)
		if fileSvc == nil {
			logger.Warnf(ctx, "[IM] rewriteStorageURLs: no file service for src=%s", match)
			return match
		}
		httpURL, err := fileSvc.GetFileURL(ctx, match)
		if err != nil {
			logger.Warnf(ctx, "[IM] rewriteStorageURLs failed: src=%s err=%v", match, err)
			return match
		}
		if httpURL == match {
			logger.Warnf(ctx,
				"[IM] rewriteStorageURLs no-op (URL unchanged; for local storage set APP_EXTERNAL_URL): src=%s",
				match)
			return match
		}
		logger.Infof(ctx, "[IM] rewriteStorageURLs: src=%s dst=%s", match, httpURL)
		return httpURL
	})
}

var incompleteURLSuffixRe = regexp.MustCompile(
	`\b(?:local|minio|s3|cos|tos|oss)://[^\s)\]>"]*$`,
)

func findIncompleteStorageURL(s string) int {
	loc := incompleteURLSuffixRe.FindStringIndex(s)
	if loc == nil {
		return -1
	}
	return loc[0]
}

var incompleteMarkdownImageSuffixRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*$`)

func findIncompleteMarkdownImage(s string) int {
	if urlIdx := findIncompleteStorageURL(s); urlIdx >= 0 {
		if imgIdx := strings.LastIndex(s[:urlIdx], "!["); imgIdx >= 0 {
			if strings.Contains(s[imgIdx:urlIdx], "](") {
				return imgIdx
			}
		}
	}
	loc := incompleteMarkdownImageSuffixRe.FindStringIndex(s)
	if loc == nil {
		return -1
	}
	return loc[0]
}

var incompleteXMLTagRe = regexp.MustCompile(
	`<(?:image|image_original|image_caption|image_ocr|kb|web)[^>]*$`,
)

func findIncompleteXMLTag(s string) int {
	loc := incompleteXMLTagRe.FindStringIndex(s)
	if loc == nil {
		return -1
	}
	return loc[0]
}

func holdbackCutoff(chunk string) int {
	cutoff := len(chunk)
	if idx := findIncompleteMarkdownImage(chunk); idx >= 0 && idx < cutoff {
		cutoff = idx
	} else if idx := findIncompleteStorageURL(chunk); idx >= 0 && idx < cutoff {
		cutoff = idx
	}
	if idx := findIncompleteXMLTag(chunk); idx >= 0 && idx < cutoff {
		cutoff = idx
	}
	return cutoff
}

func formatIMOutboundAnswer(ctx context.Context, raw string, tenant *types.Tenant, defaultFileSvc interfaces.FileService) string {
	return cleanIMContent(ctx, FormatIMDisplayContent(raw, StreamDisplayFinal), tenant, defaultFileSvc)
}

func cleanIMContent(ctx context.Context, content string, tenant *types.Tenant, defaultFileSvc interfaces.FileService) string {
	content = stripImageXMLTags(content)
	content = stripIMCitationTags(content)
	resolver := newIMFileServiceResolver(tenant, defaultFileSvc)
	content = rewriteStorageURLs(ctx, content, resolver)
	return content
}

func imLocalStorageBaseDir() string {
	baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
	if baseDir == "" {
		baseDir = "/data/files"
	}
	return baseDir
}
