#!/bin/bash
# Generates a docs-website MDX release notes file from a GitHub release.
# Usage: generate_release_notes.sh <tag>
# Requires: gh CLI authenticated, jq, python3
#
# Expected GitHub release body format:
#
#   ### New features
#   * Short description of feature one
#   * Short description of feature two
#
#   ### Bug fixes
#   * Short description of bug fix one
#
#   ### Security
#   * Short description of security fix (if any)
#
# Items under each heading populate the MDX frontmatter arrays used by
# docs-website. Full markdown detail can follow each bullet on subsequent lines.

set -e

TAG=$1
if [ -z "$TAG" ]; then
  echo "Error: tag argument required" >&2
  exit 1
fi

CLEANED_VERSION=$(echo "$TAG" | sed 's/\./-/g')
OUTPUT_FILE="infrastructure-agent-${CLEANED_VERSION}.mdx"

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
    """Return first-line bullet text from a named ### section."""
    for heading in headings:
        pattern = rf'###\s+{re.escape(heading)}\s*\n(.*?)(?=\n###|\Z)'
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
            item = item.strip()
            if item:
                items.append(item)
        return items
    return []

features = extract_section(body, 'New features', 'Features')
bugs      = extract_section(body, 'Bug fixes', 'Fixes', 'Bugfixes')
security  = extract_section(body, 'Security')

def yaml_list(items):
    if not items:
        return '[]'
    escaped = ["'" + item.replace("'", "''") + "'" for item in items]
    return '[' + ', '.join(escaped) + ']'

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
