# VHS documentation
#
# Run with:
#  vhs < .github/assets/demo.tape
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>

Output .github/assets/demo.gif
Set Shell bash
Set PlaybackSpeed 0.5
Set TypingSpeed 10ms

Set Theme "Catppuccin Macchiato"
Set FontFamily "Hack Nerd Font Mono"

Set FontSize 14
Set Width 1200
Set Height 800

Hide
Type `cd ~/Developer/KyleKing/gh-star-search` Enter Sleep 250ms
Type `go build -o /tmp/gh-star-search ./cmd/gh-start-search 2>/dev/null` Enter Sleep 2s
Type `clear` Enter
Show

# Show the help overview
Type `/tmp/gh-star-search --help` Sleep 250ms Enter
Sleep 3s

# Show database stats
Type `/tmp/gh-star-search stats` Sleep 250ms Enter
Sleep 3s

# Search for repos with a keyword
Type `/tmp/gh-star-search query --limit 5 typescript` Sleep 250ms Enter
Sleep 4s

# Show detailed info for a specific repo
Type `/tmp/gh-star-search info steipete/summarize` Sleep 250ms Enter
Sleep 4s

# List all repos
Type `/tmp/gh-star-search list --limit 8` Sleep 250ms Enter
Sleep 4s

# Show current configuration
Type `/tmp/gh-star-search config` Sleep 250ms Enter
Sleep 4s

Sleep 1s
