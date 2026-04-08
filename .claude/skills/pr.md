# PR Skill

Create a pull request for the New Relic infrastructure agent.

## Steps

1. Ensure tests pass: `make test`
2. Ensure lint passes: `make lint`
3. Check license headers: `make checklicense`
4. Push the branch and open a PR against `master`

## PR Checklist

Before opening a PR:

- [ ] `make test` passes (with race detector)
- [ ] `make lint` passes
- [ ] `make checklicense` passes (run `make addlicense` to fix)
- [ ] `make third-party-notices-check` passes if dependencies changed

## Creating the PR

```bash
gh pr create --base master --title "<title>" --body "$(cat <<'EOF'
## Summary
- <bullet points>

## Test plan
- [ ] Unit tests pass (`make test`)
- [ ] Lint passes (`make lint`)
- [ ] Tested on: Linux / macOS / Windows

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

## Notes

- Base branch is `master`
- Platform support: Linux, macOS, Windows — note which platforms were tested
- For dependency changes, regenerate third-party notices: `make third-party-notices`
