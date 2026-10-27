#!/usr/bin/env bash
# PostToolUse (Write|Edit): check only the file the agent just edited.
#   backend/**/*.go  gofmt (report only, no rewrite) + go vet of the file's package
#                    (vet compiles the package and its tests, so compile errors show too).
#   web/**/*.ts(x)   tsc diagnostics for this file; react-router typegen first when the
#                    generated .react-router/ types are missing (they are gitignored).
# Exit 2 + stderr is the only combination Claude sees.
export NO_COLOR=1 FORCE_COLOR=0 GOFLAGS="${GOFLAGS:-}"
command -v jq >/dev/null || { echo "Hook check-edited-file.sh needs jq: install it or port the hook to Node." >&2; exit 2; }

INPUT=$(cat)
FILE=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' 2>/dev/null)
CWD=$(printf '%s' "$INPUT" | jq -r '.cwd // empty' 2>/dev/null)
[ -n "$FILE" ] || exit 0
case "$FILE" in /*) ;; *) FILE="${CWD:-$PWD}/$FILE" ;; esac

case "$FILE" in
  *.go|*.ts|*.tsx) ;;
  *) exit 0 ;;
esac
[ -f "$FILE" ] || exit 0

# Checkout of the edited file, not $CLAUDE_PROJECT_DIR (stale after EnterWorktree, wrong for a
# sibling worktree). Files of other repositories are skipped; any worktree of this one is checked.
ROOT=$(git -C "$(dirname "$FILE")" rev-parse --show-toplevel 2>/dev/null) || exit 0
REPO=$(git -C "$ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)
SESSION_REPO=$(git -C "${CWD:-$PWD}" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)
[ -n "$REPO" ] && [ "$REPO" = "$SESSION_REPO" ] || exit 0

# Register the checkout so the Stop hook sweeps it too, even when it is not the session cwd.
SID=$(printf '%s' "$INPUT" | jq -r '.session_id // empty' 2>/dev/null | tr -cd 'A-Za-z0-9_-')
if [ -n "$SID" ] && mkdir -p "${TMPDIR:-/tmp}/claude-hooks"; then
  printf '%s\n' "$ROOT" >>"${TMPDIR:-/tmp}/claude-hooks/$SID.roots"
fi

REL=${FILE#"$ROOT"/}
case "$REL" in
  backend/*.go)
    command -v go >/dev/null || { echo "Hook check-edited-file.sh: go is not on PATH, $REL was not checked." >&2; exit 2; }
    cd "$ROOT/backend" || exit 0
    GOREL=${REL#backend/}
    # gofmt -l prints the file when it needs formatting, or a syntax error on stderr.
    if FMT=$(gofmt -l "$GOREL" 2>&1) && [ -z "$FMT" ]; then :; else
      echo "gofmt: $REL is not gofmt-formatted or does not parse (run gofmt -w $REL):" >&2
      { printf '%s\n' "$FMT"; gofmt -d "$GOREL" 2>/dev/null | head -n 60; } >&2
      exit 2
    fi
    if ! OUTPUT=$(go vet "./$(dirname "$GOREL")" 2>&1 </dev/null); then
      echo "go vet failed for the package of $REL:" >&2
      printf '%s\n' "$OUTPUT" >&2
      exit 2
    fi
    ;;
  web/*.ts|web/*.tsx)
    cd "$ROOT/web" || exit 0
    WREL=${REL#web/}
    if [ ! -d .react-router/types ]; then
      if ! OUTPUT=$(npx --no-install react-router typegen 2>&1 </dev/null); then
        echo "react-router typegen could not run, $REL was not typechecked:" >&2
        printf '%s\n' "$OUTPUT" | head -n 20 >&2
        exit 2
      fi
    fi
    # tsc has no single-file mode that keeps the project config: run the project and keep
    # this file's diagnostics. Other files' errors are the end-of-turn hook's job.
    OUTPUT=$(npx --no-install tsc --noEmit --pretty false 2>&1 </dev/null)
    RC=$?
    MINE=$(printf '%s\n' "$OUTPUT" | grep --color=never -F "$WREL(")
    if [ -n "$MINE" ]; then
      echo "Type errors in $REL:" >&2
      printf '%s\n' "$MINE" >&2
      exit 2
    fi
    # Failed without any TS diagnostic: the typechecker itself did not run. Say so.
    if [ "$RC" -ne 0 ] && ! printf '%s\n' "$OUTPUT" | grep -q 'error TS'; then
      echo "Typecheck could not run for $REL:" >&2
      printf '%s\n' "$OUTPUT" | head -n 20 >&2
      exit 2
    fi
    ;;
esac
exit 0
