# Zippy

Terminal speed reader that plays text one word at a time, keeping a highlighted
pivot letter centered. Keeping the word in a fixed spot reduces eye movement,
which can help increase reading speed. Even if you do not consciously register
every word, your brain can often infer meaning from context.

![zippy](https://github.com/jamestjw/zippy/raw/main/zippy.gif)

## Quick start

```bash
echo "The quick brown fox jumps over the lazy dog." | go run .
```

## Usage

```bash
go run . -file /path/to/text.txt -wpm 350
```

Use `-lazy` to stream tokens without buffering the whole input (disables back/forward).
You can also provide input via stdin by piping text into the program.

## Controls

- space: play/pause
- \+ / - or up/down: speed up/down
- h/l or left/right: step back/forward
- r: restart (file input only)
- q: quit

## Notes

- Punctuation is kept attached to words so commas/periods stay with the word as
it flashes.
- The terminal controls actual font size. Zippy does not change it.
- In `-lazy` mode, back/forward is disabled and the total word count is unknown until the stream ends.
