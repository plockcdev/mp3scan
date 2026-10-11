# Silence subcommand

## Behavior

- Start searching in the current working directory.
- Recursively find files whose extension is `.m4a`, without regard to extension case.
- Do not follow symbolic links to directories.
- Traverse directories in batches of 128 entries so a large directory is not
  retained in memory all at once. Process files in the order returned by the
  filesystem; do not collect or globally sort matching paths.
- Print each file's row as soon as that file has been processed. Do not retain
  prior result rows in memory.
- Show each filename as its full path relative to the current working directory,
  without a leading `./`.
- Read the song duration from the audio file's MP4 metadata; do not decode audio
  to determine duration.
- Show duration as `HH:MM:SS`, with hours allowed to exceed two digits.
- Require `ffprobe` and `ffmpeg` on `PATH` to inspect audio samples. If either
  executable is missing, report a single error and exit before printing a table.
- Use `ffprobe` to get the first audio stream's sample rate and channel count.
  Use `ffmpeg` to decode that stream to interleaved 32-bit floating-point PCM
  at the source sample rate and channel count, without playing the audio.
- A sample is silent when every channel's absolute amplitude is at or below
  -85 dBFS (approximately 0.00005623 on a normalized scale where full scale is 1).
- Find the longest continuous silent span ending at the final decoded sample.
  Any non-empty span qualifies as trailing silence; there is no minimum
  duration.
- Read the embedded format title with the same `ffprobe` invocation used to get
  the first audio stream's sample rate and channel count. If there is no title,
  show an empty title.
- When trailing silence exists, show the timestamp of its first sample measured
  from the start of the decoded audio, truncated to whole seconds and formatted
  as `HH:MM:SS`. Hours may exceed two digits. If the entire recording is
  silent, show `00:00:00`.
- When there is no trailing silence, show `-` for its start time.
- For `OK` and `BROKEN` rows with trailing silence, show its duration in
  seconds rounded to the nearest 0.1 second. Show `-` for `EMPTY` rows and rows
  with no trailing silence.
- For any row with trailing silence, show the lowest finite sample level in
  that span in dBFS, to two decimal places. Compute each level from the sample's
  absolute amplitude; ignore zero-amplitude samples (whose dBFS level is
  negative infinity) and non-finite sample values. Show `-` when the span has
  no non-zero finite samples or there is no trailing silence.
- Show a `Status` value for each row. Its default value is `OK`, including
  when there is no trailing silence.
- Accept the `--all` flag to include rows whose status is `OK`. Without
  `--all`, omit `OK` rows and print only rows with status `EMPTY` or `BROKEN`.
- When trailing silence starts at the first decoded sample (time zero), show
  `EMPTY`.
- Otherwise, show `OK` when the trailing-silence start is no more than three
  seconds before the end of the decoded audio, including exactly three seconds.
  Show `BROKEN` when it starts more than three seconds before decoded audio
  ends. Compare sample-accurate times before truncating the displayed timestamp.
- Show file size in kibibytes, where 1 KiB is 1024 bytes, rounded to the
  nearest 0.1 KiB. Use commas to group thousands in the integer portion.
- Keep audio processing memory bounded: stream decoded PCM through a fixed-size
  buffer and retain at most 64 KiB of `ffmpeg` error output per file.

## Output

Print a plain-text table with fixed-width columns `Duration`, `Silence starts`,
`Status`, `Silence (s)`, `Lowest silence (dBFS)`, and `Size (KiB)`, followed
by a variable-width `Title [Filename]` field. Use widths 10, 14, 8, 10, 20,
and 12 respectively; right-align each fixed-width column and separate columns
with two spaces. Do not add trailing spaces to a line. Print the header even
when no files match.

The command accepts the `--all` option to include `OK` rows. By default, omit
those rows.

Each row contains duration, trailing-silence start time or `-`, status,
trailing-silence duration or `-`, lowest finite trailing-silence level or `-`,
size in KiB, then the embedded title followed by the full relative filename in
brackets. Escape backslashes, tabs, carriage returns, and newlines in the title
and filename as `\\`, `\t`, `\r`, and `\n` so each file occupies one line. If a
matching file cannot be read, its duration cannot be determined, or its audio
cannot be decoded, report the filename and error to standard error, continue
searching, and omit that file from the output.
