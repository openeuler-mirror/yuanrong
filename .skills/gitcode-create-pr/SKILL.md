---
name: gitcode-create-pr
description: Use when creating or updating a GitCode pull request, especially for fork-to-upstream flows where head/base formatting and push order matter
---

# GitCode PR

## Overview

Create or update GitCode pull requests with the GitCode API.

Use the repository PR template at `.gitee/PULL_REQUEST_TEMPLATE/PULL_REQUEST_TEMPLATE.en.md` when preparing the PR body.

## Required Inputs

- `owner`: target repository owner
- `repo`: target repository name
- `source_branch`: branch containing the changes
- `target_branch`: branch to merge into
- `title`: PR title
- `body`: PR description in markdown

For fork PRs also collect:

- `fork_owner`
- `fork_repo`

## Commit Requirements

Commits prepared for the PR should satisfy this pattern:

```regex
^(fix|feat|docs|style|refactor|test|chore|perf|ci|build|revert)(\([^)]+\)|\[[^\]]+\])?:.+[\s\S]*Signed-off-by:.+<.+@.+>
```

Practical rules:

- The subject must start with one of `fix|feat|docs|style|refactor|test|chore|perf|ci|build|revert`
- An optional scope may use either `(scope)` or `[scope]`
- The message body must include a `Signed-off-by:` trailer
- Use `git commit -s` so the trailer is generated correctly
- Reject or rewrite commits that do not meet this format before opening the PR

## Create a PR

### 1. Push the source branch first

```bash
git push <fork-remote> <source_branch>
```

### 2. Create the PR

Same-repository PR:

```bash
curl -sS -X POST "https://gitcode.com/api/v5/repos/${owner}/${repo}/pulls" \
  -H "Content-Type: application/json" \
  -H "PRIVATE-TOKEN: ${GITCODE_APIKEY}" \
  -d "{
    \"head\": \"${source_branch}\",
    \"base\": \"${target_branch}\",
    \"title\": \"${title}\",
    \"body\": \"${body}\"
  }"
```

Cross-repository PR from a fork:

```bash
curl -sS -X POST "https://gitcode.com/api/v5/repos/${owner}/${repo}/pulls" \
  -H "Content-Type: application/json" \
  -H "PRIVATE-TOKEN: ${GITCODE_APIKEY}" \
  -d "{
    \"head\": \"${fork_owner}:${source_branch}\",
    \"base\": \"${target_branch}\",
    \"title\": \"${title}\",
    \"body\": \"${body}\",
    \"fork_path\": \"${fork_owner}/${fork_repo}\"
  }"
```

## Update a PR

```bash
curl -sS -X PATCH "https://gitcode.com/api/v5/repos/${owner}/${repo}/pulls/${pr_number}" \
  -H "Content-Type: application/json" \
  -H "PRIVATE-TOKEN: ${GITCODE_APIKEY}" \
  -d '{
    "title": "New Title",
    "body": "PR body"
  }'
```

## Notes

- GitCode fork PRs work more reliably when `head` is `fork_owner:branch`.
- `fork_path` is useful, but it does not replace the `head` fork prefix.
- If the API says the source branch is invalid, first confirm the branch exists on the fork and then re-check the `head` format.
- Before creating or updating a PR, verify the branch history does not contain commits that violate the required commit-message pattern.

## Common Mistakes

- Creating the PR before pushing the source branch
- Using `https://api.gitcode.com/api/v5` instead of `https://gitcode.com/api/v5`
- Using bare `head: "<branch>"` for fork PRs
- Forgetting the `Signed-off-by:` trailer or using a subject prefix outside the allowed set
- Omitting the repository PR template when writing the body
