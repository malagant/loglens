# LogLens demo — VHS tape
#
# Render with:  vhs docs/demo.tape   (writes docs/demo.gif)
#
# Prereqs: a `loglens` binary on PATH built from this repo. The repo's
# `make demo` recipe builds and re-renders this tape end-to-end.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 14
Set Theme "Monokai Pro"
Set TypingSpeed 35ms

# Banner
Type "# LogLens — one TUI, every log source, one query language."
Sleep 600ms
Enter
Sleep 400ms

# Tail a file source
Type "loglens --source file://docs/sample.log"
Sleep 600ms
Enter
Sleep 2500ms

# Filter live: only errors
Type "/"
Sleep 400ms
Type "level=error"
Sleep 600ms
Enter
Sleep 2200ms

# Refine: errors from payments only
Type "/"
Sleep 350ms
Type "level=error /payments/"
Sleep 600ms
Enter
Sleep 2000ms

# Inspect a row — open detail pane
Type "k"
Sleep 200ms
Type "k"
Sleep 200ms
Enter
Sleep 1800ms

# Show help overlay
Escape
Sleep 200ms
Type "?"
Sleep 2200ms
Escape
Sleep 400ms

# Quit
Type "q"
Sleep 600ms
