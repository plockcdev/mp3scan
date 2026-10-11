# Missing subcommand

## Behavior

- Recursively scan the current working directory for `.m4a` files, without
  regard to extension case, and do not follow symbolic links to directories.
- Read each file's MP4 format tags for album, album artist, artist, title,
  track number, and disc number. Use album artist when present, otherwise use
  artist. Skip a file with a missing album, artist, disc number, or track number
  after reporting the filename and reason to standard error.
- Group files by trimmed, case-insensitive artist and album names. Track
  positions are the pair (disc number, track number); parse the leading integer
  from values such as `3/4` and `6/10`.
- For each distinct album group, use the MusicBrainz web service API to search
  releases by the group's artist and album title. Retrieve candidate releases'
  complete media and track lists, including each track's medium position,
  track position, and title.
- Select a candidate only when its artist and release title match the local
  group after trimming and case-insensitive comparison, and every local track
  matches the candidate at the same disc and track positions with the same
  normalized title. Normalize titles by trimming, collapsing whitespace, and
  case-insensitive comparison.
- If no candidate matches all local tracks, or if matching candidate releases
  have different tracklists and therefore different missing-track results,
  report the album as unresolved to standard error and do not claim tracks are
  missing for it. Multiple candidate releases with identical ordered medium,
  track positions, and normalized titles are equivalent for this check.
- Compare the unique candidate tracklist with local disc/track positions. For
  each position absent locally, report the MusicBrainz track number and title.
  A duplicate local file at an already-present position counts as present.
- List only albums with one or more missing tracks. Sort album groups by
  normalized artist then album, and sort missing tracks by disc number then
  track number. Include the disc number for multi-disc releases.
- Persist successfully resolved album tracklists in the user's cache directory
  under an `mp3scanner/musicbrainz` subdirectory, using JSON files. Key each
  entry by the normalized artist and album names; cache entries are reusable
  across command runs. A cache hit must avoid MusicBrainz requests for that
  album. Do not cache failed, unresolved, or ambiguous lookups.
- Use MusicBrainz API JSON responses over HTTPS and set a meaningful
  application-and-version User-Agent that includes
  `https://github.com/plockcdev/mp3scan`, for example
  `mp3scanner/<version> (https://github.com/plockcdev/mp3scan)`. Retry each
  failed HTTP request, response decoding, or API operation up to five attempts
  total, waiting five seconds between attempts. Never make more than one API
  request per second, including retries. After the final failed attempt, report
  the affected album and error to standard error, continue with other album
  groups, and return a nonzero status if any files or albums could not be
  processed.
- A successful lookup with no missing tracks produces no album entry.

## Output

Print a plain-text report with one block per album containing missing tracks.
Each block has the artist and album title, followed by one line per missing
track in the form `Disc N, track M: Title`. Omit an album block when all of its
MusicBrainz tracks are present locally. Print a short message when no album has
missing tracks. Diagnostics and unresolved albums go to standard error.

`--help` shows this command's behavior and options. `usage` or `--usage` prints
`docs/USAGE_missing.txt`; extra arguments to either usage form are an error.
