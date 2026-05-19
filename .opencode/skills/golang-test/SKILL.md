---
name: golang-test
description: Use when running Go tests (`go test`), checking failures, race conditions, JSON test output, coverage generation, and returning a clear failed/successfully result.
---

# golang-test

## When to use

Use this skill when the task involves running Go tests and reporting the result.

Common triggers:
- "run go test"
- "test this package"
- "check failing tests"
- "generate coverage"
- "race condition in tests"

## Required command

Always run the test command in this format:

```bash
go test ./DIRECTORY_PATH/... -race -json -v -coverprofile=coverage.txt ./... 2>&1 |tee /tmp/gotest.log | gotestfmt
```

Notes:
- Replace `DIRECTORY_PATH` with the target directory relative to repo root.
- Keep `/tmp/gotest.log` as the log output path.
- Keep `coverage.txt` as the coverage output file unless the user explicitly asks for a different file.

## Result handling

After running the command:
- If output indicates failures (for example `FAIL`, non-zero exit, panic, or failed test cases), return:
  - `failed`
  - include the key error/test failure lines.
- If all tests pass, return:
  - `successfully`
  - include a short confirmation (packages tested and pass status).

## Response format

Keep responses short and objective:
- Status: `failed` or `successfully`
- Brief reason
- Key failing test/error lines when failed
