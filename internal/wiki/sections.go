package wiki

import "strings"

// wikiSection is a single heading-delimited region of a wiki page. The
// heading line itself is retained as the first line of Content so that a
// section's content fully reconstructs its source text — this is what lets
// diffSections decide a section is unchanged by comparing Content strings
// for equality.
type wikiSection struct {
	title   string
	level   int
	content string
}

// sectionDiff classifies the differences between two section lists keyed by
// heading title. Added/Modified carry the NEW section value (so callers
// re-chunk the up-to-date content); Removed carries the OLD section value.
type sectionDiff struct {
	added    []wikiSection
	modified []wikiSection
	removed  []wikiSection
}

// splitIntoSections breaks markdown content into heading-delimited sections.
// Each heading (ATX-style, '#'..'######') starts a new section; lines before
// the first heading form a single untitled (level 0) section. Content always
// includes the heading line. Empty input yields no sections.
func splitIntoSections(content string) []wikiSection {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	var sections []wikiSection

	var cur *wikiSection
	var curLines []string
	flush := func() {
		if cur == nil {
			return
		}
		cur.content = strings.Join(curLines, "\n")
		sections = append(sections, *cur)
		cur = nil
		curLines = nil
	}

	for _, line := range lines {
		if isHeading(line) {
			flush()
			level, title := parseHeading(line)
			cur = &wikiSection{title: title, level: level}
			curLines = []string{line}
			continue
		}
		// Body line: attach to the current section, starting an untitled
		// section if none exists yet (content before any heading).
		if cur == nil {
			cur = &wikiSection{title: "", level: 0}
			curLines = []string{}
		}
		curLines = append(curLines, line)
	}
	flush()
	return sections
}

// isHeading reports whether a line is an ATX markdown heading: 1-6 leading
// '#' characters followed by a space, tab, or end-of-line. Leading spaces
// are tolerated (CommonMark allows up to 3, but we accept any indent for
// resilience against hand-edited wiki content).
func isHeading(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	level := 0
	for _, ch := range trimmed {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	if level < 1 || level > 6 {
		return false
	}
	rest := trimmed[level:]
	return rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t")
}

// parseHeading extracts the heading level and trimmed title text from a
// heading line. Caller must have verified the line is a heading.
func parseHeading(line string) (level int, title string) {
	trimmed := strings.TrimLeft(line, " \t")
	for _, ch := range trimmed {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	title = strings.TrimSpace(trimmed[level:])
	return level, title
}

// diffSections compares two section lists keyed by title. A section present
// only in new is Added; present in both but differing in content or level is
// Modified (carrying the new value); present only in old is Removed. When
// duplicate titles exist within a list, the last occurrence wins for lookup.
func diffSections(old, new []wikiSection) sectionDiff {
	var diff sectionDiff

	oldByTitle := make(map[string]wikiSection, len(old))
	for _, s := range old {
		oldByTitle[s.title] = s
	}

	newTitles := make(map[string]bool, len(new))
	for _, s := range new {
		newTitles[s.title] = true
		prev, ok := oldByTitle[s.title]
		if !ok {
			diff.added = append(diff.added, s)
			continue
		}
		if prev.content != s.content || prev.level != s.level {
			diff.modified = append(diff.modified, s)
		}
	}

	for _, s := range old {
		if !newTitles[s.title] {
			diff.removed = append(diff.removed, s)
		}
	}
	return diff
}
