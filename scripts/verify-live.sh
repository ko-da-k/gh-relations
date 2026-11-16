#!/usr/bin/env bash
#
# Check that the trees in scripts/expected/ are still what the extension prints
# when it is run on the real Issues and pull requests, and that the tree the
# READMEs show is one of them.
#
# A golden test keeps the renderer and the fixture in step, but no test can see
# the other half of the claim: that the fixture still describes what is on
# GitHub. Anybody can retitle or close one of the items and leave every test
# passing. Hence this check, which needs the network and a gh token and is
# deliberately not part of pre-commit.
#
# The expectations are their own files rather than something read out of the
# README. The README is prose and gets rewritten: it held three trees and now
# holds one, and because the blocks used to be found by position, the sections
# going took two of the three checks with them without failing. A file per tree
# keeps what is checked independent of how the README is written.
#
#   lefthook run verify-live        # or: ./scripts/verify-live.sh
#   ./scripts/verify-live.sh --update   # write the live output into the files
#
set -euo pipefail

cd "$(dirname "$0")/.."

REPO=${GH_RELATIONS_FIXTURE_REPO:-ko-da-k/gh-relations}

update=0
if [ "${1:-}" = "--update" ]; then
	update=1
fi

dir=scripts/expected

# One entry per expected tree: the file's name, the argument that produces it,
# and what it is there to show.
#
#   whole    the sample graph in full: every link kind, both repositories, and
#            `+` where a limit stopped the walk. This is the tree the READMEs
#            show
#   folded   a smaller, separate graph whose point is a folded line: one pair of
#            items joined by three links at once
#   entered  the sample graph again, entered at the issue that holds the
#            sub-issues, so the same edge reads "sub of" where whole reads
#            "parent of"
#
# Indirect expansion rather than an associative array: this has to run under the
# bash 3.2 that ships with macOS.
EXPECTED=(whole folded entered)
ARG_whole=2
ARG_folded=https://github.com/$REPO/issues/1
ARG_entered=https://github.com/$REPO/issues/4

# The one the READMEs show. Every README has to agree with this file.
SHOWN=whole

READMES=(README.md README.ja.md)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
bin=$work/gh-relations

echo "==> building"
go build -o "$bin" .

# A tree drawn for a terminal carries SGR colour and OSC 8 hyperlinks. Both are
# zero width, and neither is in the expected files, so they come off before
# comparing. perl rather than sed: BSD sed has no \x1b.
strip_escapes() {
	perl -pe 's/\e\[[0-9;]*m//g; s/\e\]8;;[^\e]*\e\\//g'
}

# GH_FORCE_TTY makes go-gh report a terminal without one, which is how the
# colour form is reproduced in a pipe. It is not a number and has no % suffix,
# so it sets no width override and nothing is truncated.
#
# NO_COLOR and CLICOLOR are cleared for the same reason: this helper exists to
# produce the colour form, and either of them would turn the state back into a
# word and make every tree look like a mismatch. `lefthook run --colors off`
# sets NO_COLOR in the jobs it runs, so this is not hypothetical.
drawn() {
	env -u NO_COLOR -u CLICOLOR GH_FORCE_TTY=true "$bin" "$@" --repo "$REPO" 2>/dev/null | strip_escapes
}

# console_block FILE prints the body of the first ```console block in FILE
# without its first line, which is the prompt rather than output. The block is
# found by its language tag, so a block added above it changes nothing.
console_block() {
	awk '
		/^```console/ { if (!seen) { seen = 1; inblock = 1; next } }
		inblock && /^```/ { inblock = 0 }
		inblock { print }
	' "$1" | tail -n +2
}

# Every 【for Test】 title written into the prose, outside the fenced blocks.
prose_titles() {
	awk '
		/^```/ { open = !open; next }
		!open {
			line = $0
			while (match(line, /【for Test】[^`]*/)) {
				print substr(line, RSTART, RLENGTH)
				line = substr(line, RSTART + RLENGTH)
			}
		}
	' "$1"
}

fail=0

compare() {
	local what=$1 want=$2 got=$3
	if diff -u "$want" "$got" > "$work/diff"; then
		echo "    ok   $what"
	else
		echo "    FAIL $what"
		sed 's/^/         /' "$work/diff"
		fail=1
	fi
}

echo "==> reading $REPO"

for name in "${EXPECTED[@]}"; do
	arg=ARG_$name
	drawn "${!arg}" > "$work/$name"
	if [ "$update" -eq 1 ]; then
		if cmp -s "$work/$name" "$dir/$name.txt"; then
			echo "    same $dir/$name.txt"
		else
			cp "$work/$name" "$dir/$name.txt"
			echo "    wrote $dir/$name.txt"
		fi
		continue
	fi
	compare "$dir/$name.txt" "$dir/$name.txt" "$work/$name"
done

if [ "$update" -eq 1 ]; then
	echo "==> expected trees written from $REPO; review the diff before recording"
	exit 0
fi

# The README is prose, but the tree in it is program output, so it has to be one
# of the expectations rather than something typed by hand. Local: no requests,
# and the expectations above are what reaches GitHub.
for doc in "${READMES[@]}"; do
	console_block "$doc" > "$work/shown"
	compare "$doc: the tree it shows" "$dir/$SHOWN.txt" "$work/shown"
done

# An inline example in the prose carries a title too, and nothing above compares
# it: what is compared is the block. A prose title that none of the expected
# trees holds is one a rename left behind, which is how a stale title survived a
# retitle once.
#
# Only the title is matched, not the whole example: the prose writes the state
# inline ("#3 closed: ...") where the colour form carries it as colour. A title
# long enough to be truncated in a tree would fail here, which no title in
# either file currently is.
cat "$dir"/*.txt > "$work/alltrees"
for doc in "${READMES[@]}"; do
	prose_titles "$doc" > "$work/titles"
	while IFS= read -r t; do
		if grep -qF -- "$t" "$work/alltrees"; then
			echo "    ok   $doc: inline example \"$t\""
		else
			echo "    FAIL $doc: no expected tree holds the inline example \"$t\""
			fail=1
		fi
	done < "$work/titles"
done

if [ "$fail" -ne 0 ]; then
	cat <<'EOF'

The expected trees and the real items no longer agree.

  - If the items changed, put them back, or rebuild the fixture from them and
    then run this with --update (tmp/live/ holds the scripts that made them).
  - If the renderer or the fixture changed on purpose, run this with --update
    and read the diff before recording it.
  - If a README failed, its tree is no longer the expected one. Copy the tree
    from scripts/expected/ instead of editing it by hand.
  - If an inline example failed, a title was renamed and the prose kept the old
    one. Take the new title from scripts/expected/.
  - If this failed without reaching GitHub, check `gh auth status` first: a tree
    that comes back empty is not a mismatch.
EOF
	exit 1
fi

echo "==> every expected tree matches $REPO, and both READMEs show one of them"
