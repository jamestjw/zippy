# Zippy

Terminal speed reader that plays text one word at a time, keeping a highlighted pivot letter centered.

## Quick start

```bash
echo "The quick brown fox jumps over the lazy dog." | go1.25.6 run .
```

## Usage

```bash
go1.25.6 run . -file /path/to/text.txt -start-wpm 350
```

If you prefer, `-wpm` is an alias for `-start-wpm`.

## Controls

- space: play/pause
- + / - or up/down: speed up/down
- h/l or left/right: step back/forward
- q: quit

## Notes

- Punctuation is kept attached to words so commas/periods stay with the word as it flashes.
- The terminal controls actual font size; Zippy increases perceived size by adding letter spacing and vertical repetition.
