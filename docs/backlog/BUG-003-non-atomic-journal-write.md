# BUG-003: Journal write is not atomic

- **Type:** bug
- **Priority:** P1 — can corrupt the journal
- **Status:** open
- **Area:** `internal/journal/journal.go` (`Replace`), `internal/app/app.go`
  (`finalizeTransaction`, `validateCandidateContent`)

## Symptom

`journal.Replace` calls `os.WriteFile(path, content, 0o644)`, which opens the
existing journal with `O_TRUNC` and then writes the full new content in place.

## Failure scenarios

1. **Disk full (ENOSPC).** The file is truncated to zero, the write fails part
   way, `Replace` returns an error, and the journal is left partial. The
   validated candidate has already been removed by `defer os.Remove`, so no
   intact copy exists.
2. **Crash, kill or power loss** between truncate and the end of the write gives
   the same result.
3. **Concurrent external edit.** The original is read in `candidateAppend`,
   validated (about 0.5 s or more with a real journal), then overwritten. Edits
   an editor saves in that window are lost silently.

Product principle 5 ("a failed or cancelled interaction must not modify the
journal") is guaranteed only when writes never fail.

## Proposed fix

1. Resolve the target with `filepath.EvalSymlinks`, so a symlinked journal
   updates its real file instead of replacing the link.
2. Write the candidate into a temp file in the **same directory** (already done
   for validation), `Sync()`, `Close()`.
3. `Chmod` it to the original file's mode (`os.CreateTemp` creates `0600`).
4. Before renaming, check that the original is unchanged (size + mtime, or a
   content hash taken at read time). If it changed, abort with a clear message.
5. `os.Rename` the **validated** temp file over the target, then fsync the
   directory. What was validated is then byte-for-byte what lands.
6. On any failure remove the temp file; the original stays untouched.

## Acceptance criteria

- Tests: the original is intact when the rename step fails; the file mode is
  preserved; a symlink keeps pointing to the updated file; a change to the
  original between read and write aborts the write.
- `defer os.Remove` no longer deletes the only valid copy before the write
  succeeds.

## Related

- BUG-004 (temp candidate handling), ENG-001 (integration test of write path).
