#!/bin/bash
# Generates a docs-website MDX release notes file from a GitHub release.
# Usage: generate_release_notes.sh <tag>
# Requires: gh CLI authenticated, jq, python3
#
# Expected GitHub release body format:
#
#   ### 🚀 Enhancements
#   - Short description of feature one
#
#   ### 🐞 Bug fixes
#   - Short description of bug fix one
#
#   ### 🛡️ Security notices
#   - Short description of security fix (if any)
#
# Heading matching is substring-based (case-insensitive) so emoji-prefixed
# headers like "### 🚀 Enhancements" still match. Items under Enhancements /
# Bug fixes / Security notices populate the MDX frontmatter arrays (features /
# bugs / security) used by docs-website; trailing PR/commit refs (#123) and
# any "in <path>" qualifier (common on dependency-bump lines) are stripped.
# The full release body (with "## What's Changed" renamed to "## Notes" and
# the "Full Changelog: ..." line removed) is still appended below the
# frontmatter as-is.

set -e

TAG=$1
if [ -z "$TAG" ]; then
  echo "Error: tag argument required" >&2
  exit 1
fi

VERSION_NODOTS=$(echo "$TAG" | tr -d '.')
OUTPUT_FILE="new-relic-infrastructure-agent-${VERSION_NODOTS}.mdx"

RELEASE_INFO=$(gh release view "$TAG" --json publishedAt,body)
RELEASE_DATE=$(echo "$RELEASE_INFO" | jq -r '.publishedAt | split("T")[0]')

export TAG RELEASE_DATE OUTPUT_FILE
export RELEASE_BODY=$(echo "$RELEASE_INFO" | jq -r '.body' | sed 's/\r//')

python3 << 'PYEOF'
import re, os

body         = os.environ['RELEASE_BODY']
tag          = os.environ['TAG']
release_date = os.environ['RELEASE_DATE']
output_file  = os.environ['OUTPUT_FILE']

def extract_section(text, *headings):
    """Return bullet items from a ### section, matched by heading substring
    (handles emoji-prefixed headers like '### 🚀 Enhancements')."""
    for heading in headings:
        # stop at the next heading of ANY level (not just another ###), so a
        # trailing "## Notes" / "## What's Changed" section can't be slurped in
        pattern = rf'###[^\n]*{re.escape(heading)}[^\n]*\n(.*?)(?=\n#{{1,6}}\s|\Z)'
        match = re.search(pattern, text, re.DOTALL | re.IGNORECASE)
        if not match:
            continue
        items = []
        for line in match.group(1).splitlines():
            line = line.strip()
            if not line.startswith(('* ', '- ')):
                continue
            item = line[2:].strip()
            # strip trailing PR/commit refs: (#123) or (abc1234)
            item = re.sub(r'\s*\(#\d+\)\s*$', '', item)
            item = re.sub(r'\s*\([0-9a-f]{7,40}\)\s*$', '', item)
            # drop trailing qualifiers like "in /some/path" (common on dependency bumps)
            item = re.sub(r'\s+in\s+.*$', '', item, flags=re.IGNORECASE)
            item = item.strip()
            if item:
                items.append(item)
        return items
    return []

features = extract_section(body, 'Enhancements', 'New features', 'Features')
bugs      = extract_section(body, 'Bug fixes', 'Fixes', 'Bugfixes')
security  = extract_section(body, 'Security notices', 'Security')

def yaml_list(items):
    if not items:
        return '[]'
    escaped = ["'" + item.replace("'", "''") + "'" for item in items]
    return '[' + ', '.join(escaped) + ']'

def clean_body(text):
    text = re.sub(r"^##\s*What's Changed\s*$", '## Notes', text, flags=re.MULTILINE | re.IGNORECASE)
    lines = [
        line for line in text.splitlines()
        if not re.match(r'^\*{0,2}Full Changelog\*{0,2}:', line.strip(), re.IGNORECASE)
    ]
    return '\n'.join(lines)

def linkify_pr_urls(text):
    """Turn a bare PR URL (as GitHub's auto-generated 'in <url>' references
    render) into a markdown link showing just #NNNN as the visible text.
    Skips URLs already inside a markdown link (preceded by "](") so an
    already-formatted [#NNNN](url)""
    pattern = re.compile(r'(?<!\]\()https://github\.com/[\w.-]+/[\w.-]+/pull/(\d+)')
    return pattern.sub(lambda m: f'[#{m.group(1)}]({m.group(0)})', text)

def normalize_spacing(text):
    """Force exactly one blank line both before and after every heading
    (regardless of how the release was authored — heading-into-content,
    content-into-heading, or double blank lines between sections), while
    leaving other paragraph spacing untouched."""
    lines = text.splitlines()
    heading_re = re.compile(r'^#{1,6}\s')
    spaced = []
    i = 0
    n = len(lines)
    while i < n:
        line = lines[i]
        if heading_re.match(line):
            if spaced and spaced[-1].strip() != '':
                spaced.append('')
            spaced.append(line)
            j = i + 1
            while j < n and lines[j].strip() == '':
                j += 1
            if j < n:
                spaced.append('')
            i = j
            continue
        spaced.append(line)
        i += 1

    collapsed = []
    blank_run = 0
    for line in spaced:
        if line.strip() == '':
            blank_run += 1
            if blank_run <= 1:
                collapsed.append(line)
        else:
            blank_run = 0
            collapsed.append(line)
    while collapsed and collapsed[0].strip() == '':
        collapsed.pop(0)
    while collapsed and collapsed[-1].strip() == '':
        collapsed.pop()
    return '\n'.join(collapsed)

body = clean_body(body)
body = linkify_pr_urls(body)
body = normalize_spacing(body)

with open(output_file, 'w') as f:
    f.write(f"""\
---
subject: Infrastructure agent
releaseDate: '{release_date}'
version: {tag}
features: {yaml_list(features)}
bugs: {yaml_list(bugs)}
security: {yaml_list(security)}
---

{body}
""")

print(f'Generated: {output_file}')
PYEOF

echo "Generated release notes: $OUTPUT_FILE"
