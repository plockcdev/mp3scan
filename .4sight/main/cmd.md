# Main command

## Behavior

- Start searching in the current working directory.
- Recursively find files whose extension is `.m4a`, without regard to extension case.
- Do not follow symbolic links to directories.
- Print one row for each matching file, ordered by relative filename.
- Show each filename relative to the current working directory.
- Read the song duration from the audio file's metadata; do not play the audio.
- Show duration as `HH:MM:SS`, with hours allowed to exceed two digits.
- Require `ffprobe` and `ffmpeg` on `PATH` to inspect audio samples. If either
  executable is missing, report a single error and exit before printing a table.
- Use `ffprobe` to get the first audio stream's sample rate and channel count.
  Use `ffmpeg` to decode that stream to interleaved 32-bit floating-point PCM
  at the source sample rate and channel count, without playing the audio.
- A sample is silent when every channel's absolute amplitude is at or below
  -70 dBFS (approximately 0.00031623 on a normalized scale where full scale is 1).
- Find the longest continuous silent span ending at the final decoded sample.
  Count it as trailing silence only when it lasts at least 500 milliseconds.
- When trailing silence exists, show the timestamp of its first sample measured
  from the start of the decoded audio, rounded to the nearest millisecond, as
  `HH:MM:SS.mmm`. Hours may exceed two digits. If the entire recording is silent
  and lasts at least 500 milliseconds, show `00:00:00.000`.
- When there is no qualifying trailing silence, show `-` for its start time.
- Show a `Status` value for each row. Its default value is `OK`, including
  when there is no qualifying trailing silence.
- When qualifying trailing silence starts at the first decoded sample (time
  zero), show `EMPTY`.
- Otherwise, show `OK` when the trailing-silence start is no more than one
  second before the end of the decoded audio, including exactly one second.
  Show `BROKEN` when it starts more than one second before the decoded audio
  ends. Compare the sample-accurate start and end times before rounding the
  displayed timestamp to milliseconds.
- Show file size as an integer number of bytes.

## Output

Print a plain-text table with the header columns `Filename`, `Duration`,
`Silence starts`, `Status`, and `Size (bytes)` in that order. Print the header
even when no files match.
Set each column's width to the widest header or value in that column. Left-align
filenames and right-align durations, silence timestamps, and sizes. Separate
adjacent columns with two spaces, and do not add trailing spaces to a line.
Escape backslashes, tabs,
carriage returns, and newlines in filenames as `\\`, `\t`, `\r`, and `\n`
before measuring their width so each file occupies one line.

Each row contains the relative filename, duration, trailing-silence start time
or `-`, status, and size in that order. If a matching file cannot be read, its
duration cannot be determined, or its audio cannot be decoded, report the
filename and error to standard error, continue searching, and omit that file
from the table.
