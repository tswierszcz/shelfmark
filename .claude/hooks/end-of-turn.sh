#!/usr/bin/env bash
# Stop: sweep everything this turn changed before the agent finishes; one retry.
#   Go:  gofmt on changed .go files, go vet ./..., go build ./..., go test of the changed
#        packages. internal/catalog is left out (its integration tests start a Postgres
#        container per test, ~2 min) and the skip is reported, never silent.
#   Web: react-router typegen when missing, then tsc for the whole web project, then
#        `vitest run` once tsc is clean (when web/vitest.config.ts exists). Tests not run
#        because tsc failed, and a missing vitest binary, are reported, never silent.
# Also catches files rewritten through a shell command, which never reach the per-edit hook.
export NO_COLOR=1 FORCE_COLOR=0
command -v jq >/dev/null || { echo "Hook end-of-turn.sh needs jq: install it or port the hook to Node." >&2; exit 2; }

INPUT=$(cat)

# Already sent back once by this hook: let it finish. The commit gate catches the rest.
# (loop_count covers Cursor, which imports this hook from .claude/settings.json.)
ACTIVE=$(printf '%s' "$INPUT" | jq -r '.stop_hook_active // false' 2>/dev/null)
LOOPS=$(printf '%s' "$INPUT" | jq -r '.loop_count // 0' 2>/dev/null)
if [ "$ACTIVE" = "true" ] || [ "${LOOPS:-0}" != "0" ]; then
  exit 0
fi

# Checkouts to sweep: the session's (payload cwd follows EnterWorktree) plus every checkout
# check-edited-file.sh registered for this session.
CWD=$(printf '%s' "$INPUT" | jq -r '.cwd // empty' 2>/dev/null)
SID=$(printf '%s' "$INPUT" | jq -r '.session_id // empty' 2>/dev/null | tr -cd 'A-Za-z0-9_-')
REGISTRY="${TMPDIR:-/tmp}/claude-hooks/${SID:-none}.roots"
ROOTS=()
while IFS= read -r r; do
  [ -n "$r" ] && [ -d "$r" ] && ROOTS+=("$r")
done <<EOF
$({ git -C "${CWD:-$PWD}" rev-parse --show-toplevel 2>/dev/null; [ -n "$SID" ] && cat "$REGISTRY" 2>/dev/null; } | sort -u)
EOF
if [ "${#ROOTS[@]}" -eq 0 ]; then
  jq -cn --arg m "end-of-turn.sh: no git checkout at ${CWD:-$PWD}, nothing checked." '{systemMessage: $m}'
  exit 0
fi

SKIPPED=""
REPORT=""
for ROOT in "${ROOTS[@]}"; do
  cd "$ROOT" || continue

  # Changed and new files. Nothing changed (a Q&A turn): nothing to check here.
  CHANGED=$({ git diff --name-only HEAD; git ls-files -o --exclude-standard; } 2>/dev/null | sort -u)
  [ -n "$CHANGED" ] || continue

  GO_CHANGED=$(printf '%s\n' "$CHANGED" | grep --color=never -E '^backend/(.*\.go|go\.mod|go\.sum)$')
  WEB_CHANGED=$(printf '%s\n' "$CHANGED" | grep --color=never -E '^web/.*\.(ts|tsx|json)$')

  if [ -n "$GO_CHANGED" ]; then
    if ! command -v go >/dev/null; then
      REPORT="$REPORT
go is not on PATH: Go changes in $ROOT were not checked.
"
    else
      (
        cd backend || exit 0
        FILES=()
        PKGS=()
        while IFS= read -r f; do
          case "$f" in
            backend/*.go)
              g=${f#backend/}
              [ -f "$g" ] && FILES+=("$g")
              PKGS+=("./$(dirname "$g")")
              ;;
          esac
        done <<<"$GO_CHANGED"
        if [ "${#FILES[@]}" -gt 0 ]; then
          FMT=$(gofmt -l "${FILES[@]}" 2>&1)
          [ -n "$FMT" ] && printf '%s\n' "gofmt: not formatted or does not parse (run gofmt -w):" "$FMT" ""
        fi
        OUT=$(go vet ./... 2>&1 </dev/null) || printf '%s\n' "go vet ./... fails:" "$OUT" ""
        OUT=$(go build ./... 2>&1 </dev/null) || printf '%s\n' "go build ./... fails:" "$OUT" ""
        TEST_PKGS=()
        while IFS= read -r p; do
          [ -n "$p" ] || continue
          case "$p" in
            ./internal/catalog|./internal/catalog/*) echo "@@SKIP@@" ;;
            *) [ -d "$p" ] && TEST_PKGS+=("$p") ;;
          esac
        done < <(printf '%s\n' "${PKGS[@]}" | sort -u)
        if [ "${#TEST_PKGS[@]}" -gt 0 ]; then
          OUT=$(go test "${TEST_PKGS[@]}" 2>&1 </dev/null) || printf '%s\n' "go test ${TEST_PKGS[*]} fails:" "$OUT" ""
        fi
      ) >"${TMPDIR:-/tmp}/end-of-turn.$$" 2>&1
      GO_OUT=$(cat "${TMPDIR:-/tmp}/end-of-turn.$$"); rm -f "${TMPDIR:-/tmp}/end-of-turn.$$"
      if printf '%s\n' "$GO_OUT" | grep -q '^@@SKIP@@$'; then
        SKIPPED="$SKIPPED internal/catalog tests (~2 min, Postgres per test; run 'go test ./internal/catalog/...' before committing);"
        GO_OUT=$(printf '%s\n' "$GO_OUT" | grep --color=never -v '^@@SKIP@@$')
      fi
      [ -n "$(printf '%s' "$GO_OUT" | tr -d '[:space:]')" ] && REPORT="$REPORT
Go checks fail ($ROOT):
$GO_OUT
"
    fi
  fi

  if [ -n "$WEB_CHANGED" ] && [ -d web ]; then
    WEB_OUT=$(
      cd web || exit 0
      if [ ! -d .react-router/types ]; then
        OUT=$(npx --no-install react-router typegen 2>&1 </dev/null) || { printf '%s\n' "react-router typegen could not run:" "$OUT"; exit 0; }
      fi
      OUT=$(npx --no-install tsc --noEmit --pretty false 2>&1 </dev/null) || printf '%s\n' "$OUT"
    )
    [ -n "$WEB_OUT" ] && REPORT="$REPORT
Web typecheck fails ($ROOT/web):
$WEB_OUT
"
    if [ -n "$WEB_OUT" ] && [ -f web/vitest.config.ts ]; then
      REPORT="${REPORT}Web tests not run ($ROOT/web): typecheck failed.
"
    elif [ -f web/vitest.config.ts ]; then
      TEST_OUT=$(cd web && npx --no-install vitest run --reporter=dot 2>&1 </dev/null)
      if [ $? -ne 0 ]; then
        if printf '%s\n' "$TEST_OUT" | grep -q 'could not determine executable'; then
          SKIPPED="$SKIPPED web tests (vitest not installed; run 'npm ci' in web/);"
        else
          REPORT="$REPORT
Web tests fail ($ROOT/web):
$(printf '%s\n' "$TEST_OUT" | tail -n 80)
"
        fi
      fi
    fi
  fi
done

if [ -n "$REPORT" ]; then
  [ -n "$SKIPPED" ] && REPORT="$REPORT
Skipped:$SKIPPED
"
  echo "Fix these before you finish:$REPORT" >&2
  exit 2
fi
# Green: the registered checkouts are clean, start the next turn with an empty registry.
[ -n "$SID" ] && : >"$REGISTRY" 2>/dev/null
[ -n "$SKIPPED" ] && jq -cn --arg m "end-of-turn.sh passed, but skipped:$SKIPPED" '{systemMessage: $m}'
exit 0
