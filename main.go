package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const rootUsagePath = "docs/USAGE.txt"
const silenceUsagePath = "docs/USAGE_silence.txt"
const missingUsagePath = "docs/USAGE_missing.txt"

type atom struct {
	typ     string
	payload int64
	end     int64
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return printRootHelp(os.Stdout)
	}
	if args[0] == "-h" || args[0] == "--help" {
		if len(args) != 1 {
			if err := printRootHelp(os.Stderr); err != nil {
				return err
			}
			return errors.New("help cannot be combined with other arguments")
		}
		return printRootHelp(os.Stdout)
	}
	if args[0] == "usage" || args[0] == "--usage" {
		if len(args) != 1 {
			if err := printRootHelp(os.Stderr); err != nil {
				return err
			}
			return errors.New("usage does not accept additional arguments")
		}
		return printUsage(rootUsagePath)
	}

	switch args[0] {
	case "silence":
		return runSilence(args[1:])
	case "missing":
		return runMissing(args[1:])
	default:
		if err := printRootHelp(os.Stderr); err != nil {
			return err
		}
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printRootHelp(w io.Writer) error {
	_, err := io.WriteString(w, `mp3scanner - scan M4A files

Commands:
  silence     Report duration, trailing silence, and file size
  missing     List missing tracks for albums

Arguments:
  command     silence or missing

Options:
  -h, --help  Show help
  --usage     Show practical usage examples
`)
	return err
}

func printSilenceHelp(w io.Writer) error {
	_, err := io.WriteString(w, `mp3scanner silence - report duration and trailing silence for M4A files

Arguments:
  (none)

Options:
  -h, --help  Show help
  --usage     Show practical usage examples
  --all       Include rows with OK status (default: EMPTY and BROKEN only)
`)
	return err
}

func printMissingHelp(w io.Writer) error {
	_, err := io.WriteString(w, `mp3scanner missing - find missing album tracks using MusicBrainz

Arguments:
  (none)

Options:
  -h, --help  Show help
  --usage     Show practical usage examples
`)
	return err
}

func printUsage(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	_, err = os.Stdout.Write(data)
	return err
}

func runSilence(args []string) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			if len(args) != 1 {
				if err := printSilenceHelp(os.Stderr); err != nil {
					return err
				}
				return errors.New("help cannot be combined with other arguments")
			}
			return printSilenceHelp(os.Stdout)
		}
	}
	if len(args) == 1 && (args[0] == "usage" || args[0] == "--usage") {
		return printUsage(silenceUsagePath)
	}
	includeOK := len(args) == 1 && args[0] == "--all"
	if len(args) > 0 && !includeOK {
		if err := printSilenceHelp(os.Stderr); err != nil {
			return err
		}
		if args[0] == "usage" || args[0] == "--usage" {
			return errors.New("usage does not accept additional arguments")
		}
		return fmt.Errorf("unexpected argument %q", args[0])
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return errors.New("ffprobe is required on PATH")
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg is required on PATH")
	}
	return scanCurrentDirectory(os.Stdout, os.Stderr, ffprobePath, ffmpegPath, includeOK)
}

func runMissing(args []string) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			if len(args) != 1 {
				if err := printMissingHelp(os.Stderr); err != nil {
					return err
				}
				return errors.New("help cannot be combined with other arguments")
			}
			return printMissingHelp(os.Stdout)
		}
	}
	if len(args) == 1 && (args[0] == "usage" || args[0] == "--usage") {
		return printUsage(missingUsagePath)
	}
	if len(args) > 0 {
		if err := printMissingHelp(os.Stderr); err != nil {
			return err
		}
		if args[0] == "usage" || args[0] == "--usage" {
			return errors.New("usage does not accept additional arguments")
		}
		return fmt.Errorf("unexpected argument %q", args[0])
	}
	return runMissingScan(os.Stdout, os.Stderr, os.Getwd)
}

func scanCurrentDirectory(stdout, stderr io.Writer, ffprobePath, ffmpegPath string, includeOK bool) error {
	if _, err := fmt.Fprintf(stdout, "%10s  %14s  %8s  %10s  %20s  %12s  %s\n",
		"Duration", "Silence starts", "Status", "Silence (s)", "Lowest silence (dBFS)", "Size (KiB)", "Title [Filename]"); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	fileErrors := 0
	walkErr := walkDirStreaming(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == "." {
				return walkErr
			}
			if _, err := fmt.Fprintf(stderr, "%s: %v\n", path, walkErr); err != nil {
				return err
			}
			fileErrors++
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".m4a") {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			if _, printErr := fmt.Fprintf(stderr, "%s: %v\n", path, err); printErr != nil {
				return printErr
			}
			fileErrors++
			return nil
		}
		duration, err := readDuration(path)
		if err != nil {
			if _, printErr := fmt.Fprintf(stderr, "%s: %v\n", path, err); printErr != nil {
				return printErr
			}
			fileErrors++
			return nil
		}
		silence, err := trailingSilence(path, ffprobePath, ffmpegPath)
		if err != nil {
			if _, printErr := fmt.Fprintf(stderr, "%s: %v\n", path, err); printErr != nil {
				return printErr
			}
			fileErrors++
			return nil
		}
		relativePath, err := filepath.Rel(".", path)
		if err != nil {
			if _, printErr := fmt.Fprintf(stderr, "%s: %v\n", path, err); printErr != nil {
				return printErr
			}
			fileErrors++
			return nil
		}
		relativePath = strings.TrimPrefix(filepath.Clean(relativePath), "."+string(filepath.Separator))
		if silence.status == "OK" && !includeOK {
			return nil
		}
		if _, err := fmt.Fprintf(stdout, "%10s  %14s  %8s  %10s  %20s  %12s  %s [%s]\n",
			duration, silence.start, silence.status, silence.duration, silence.lowestLevel,
			formatSizeKiB(info.Size()), escapeField(silence.title), escapeFilename(relativePath)); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("search current directory: %w", walkErr)
	}
	if fileErrors > 0 {
		return fmt.Errorf("could not process %d path(s)", fileErrors)
	}
	return nil
}

func formatSizeKiB(sizeBytes int64) string {
	whole := sizeBytes / 1024
	tenths := ((sizeBytes%1024)*10 + 512) / 1024
	if tenths == 10 {
		whole++
		tenths = 0
	}
	return fmt.Sprintf("%s.%d", commaGroup(whole), tenths)
}

func commaGroup(value int64) string {
	digits := strconv.FormatInt(value, 10)
	firstGroup := len(digits) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	var grouped strings.Builder
	grouped.Grow(len(digits) + (len(digits)-1)/3)
	grouped.WriteString(digits[:firstGroup])
	for offset := firstGroup; offset < len(digits); offset += 3 {
		grouped.WriteByte(',')
		grouped.WriteString(digits[offset : offset+3])
	}
	return grouped.String()
}

const directoryReadBatchSize = 128

func walkDirStreaming(root string, visit func(path string, entry fs.DirEntry, err error) error) error {
	info, err := os.Lstat(root)
	if err != nil {
		return visit(root, nil, err)
	}
	entry := fs.FileInfoToDirEntry(info)
	if err := visit(root, entry, nil); err != nil || !entry.IsDir() {
		return err
	}
	return walkDirectory(root, entry, visit)
}

func walkDirectory(path string, entry fs.DirEntry, visit func(path string, entry fs.DirEntry, err error) error) (retErr error) {
	directory, err := os.Open(path)
	if err != nil {
		return visit(path, entry, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil && retErr == nil {
			retErr = visit(path, entry, closeErr)
		}
	}()

	for {
		entries, readErr := directory.ReadDir(directoryReadBatchSize)
		for _, child := range entries {
			childPath := filepath.Join(path, child.Name())
			if err := visit(childPath, child, nil); err != nil {
				return err
			}
			if child.IsDir() {
				if err := walkDirectory(childPath, child, visit); err != nil {
					return err
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return visit(path, entry, readErr)
		}
	}
}

var filenameEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"\t", "\\t",
	"\r", "\\r",
	"\n", "\\n",
)

func escapeFilename(filename string) string {
	return escapeField(filename)
}

func escapeField(value string) string {
	return filenameEscaper.Replace(value)
}

func readDuration(path string) (formatted string, retErr error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("close %s: %w", path, err)
		}
	}()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	duration, found, err := findAudioDuration(f, info.Size())
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("no audio track duration found")
	}
	if duration.timescale == 0 {
		return "", errors.New("audio duration has a zero timescale")
	}
	seconds := duration.value / duration.timescale
	hours := seconds / 3600
	minutes := (seconds / 60) % 60
	remainingSeconds := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, remainingSeconds), nil
}

type mediaDuration struct {
	value     uint64
	timescale uint64
}

func findAudioDuration(r io.ReaderAt, size int64) (mediaDuration, bool, error) {
	var result mediaDuration
	found := false
	err := eachAtom(r, 0, size, func(a atom) error {
		if a.typ != "moov" {
			return nil
		}
		return eachAtom(r, a.payload, a.end, func(track atom) error {
			if track.typ != "trak" {
				return nil
			}
			return eachAtom(r, track.payload, track.end, func(media atom) error {
				if media.typ != "mdia" {
					return nil
				}
				info, handler, hasHeader, err := parseMedia(r, media)
				if err != nil {
					return err
				}
				if hasHeader && handler == "soun" {
					result = info
					found = true
					return errStopAtoms
				}
				return nil
			})
		})
	})
	if errors.Is(err, errStopAtoms) {
		return result, found, nil
	}
	return result, found, err
}

var errStopAtoms = errors.New("stop atom iteration")

func parseMedia(r io.ReaderAt, a atom) (mediaDuration, string, bool, error) {
	var duration mediaDuration
	handler := ""
	hasHeader := false
	err := eachAtom(r, a.payload, a.end, func(child atom) error {
		switch child.typ {
		case "hdlr":
			data := make([]byte, 12)
			if _, err := r.ReadAt(data, child.payload); err != nil {
				return fmt.Errorf("read hdlr atom: %w", err)
			}
			handler = string(data[8:12])
		case "mdhd":
			parsed, err := parseMediaHeader(r, child)
			if err != nil {
				return err
			}
			duration = parsed
			hasHeader = true
		}
		return nil
	})
	return duration, handler, hasHeader, err
}

func parseMediaHeader(r io.ReaderAt, a atom) (mediaDuration, error) {
	if a.end-a.payload < 4 {
		return mediaDuration{}, errors.New("short mdhd atom")
	}
	var versionByte [1]byte
	if _, err := r.ReadAt(versionByte[:], a.payload); err != nil {
		return mediaDuration{}, fmt.Errorf("read mdhd version: %w", err)
	}
	switch versionByte[0] {
	case 0:
		data := make([]byte, 20)
		if a.end-a.payload < int64(len(data)) {
			return mediaDuration{}, errors.New("short version 0 mdhd atom")
		}
		if _, err := r.ReadAt(data, a.payload); err != nil {
			return mediaDuration{}, fmt.Errorf("read version 0 mdhd atom: %w", err)
		}
		return mediaDuration{
			timescale: uint64(binary.BigEndian.Uint32(data[12:16])),
			value:     uint64(binary.BigEndian.Uint32(data[16:20])),
		}, nil
	case 1:
		data := make([]byte, 32)
		if a.end-a.payload < int64(len(data)) {
			return mediaDuration{}, errors.New("short version 1 mdhd atom")
		}
		if _, err := r.ReadAt(data, a.payload); err != nil {
			return mediaDuration{}, fmt.Errorf("read version 1 mdhd atom: %w", err)
		}
		return mediaDuration{
			timescale: uint64(binary.BigEndian.Uint32(data[20:24])),
			value:     binary.BigEndian.Uint64(data[24:32]),
		}, nil
	default:
		return mediaDuration{}, fmt.Errorf("unsupported mdhd version %d", versionByte[0])
	}
}

func eachAtom(r io.ReaderAt, start, end int64, visit func(atom) error) error {
	for pos := start; pos < end; {
		if end-pos < 8 {
			return errors.New("truncated atom header")
		}
		var header [16]byte
		if _, err := r.ReadAt(header[:8], pos); err != nil {
			return fmt.Errorf("read atom header: %w", err)
		}
		size := uint64(binary.BigEndian.Uint32(header[0:4]))
		typ := string(header[4:8])
		headerSize := uint64(8)
		switch size {
		case 0:
			size = uint64(end - pos)
		case 1:
			if end-pos < 16 {
				return errors.New("truncated extended atom header")
			}
			if _, err := r.ReadAt(header[8:16], pos+8); err != nil {
				return fmt.Errorf("read extended atom header: %w", err)
			}
			size = binary.BigEndian.Uint64(header[8:16])
			headerSize = 16
		}
		if size < headerSize || size > uint64(end-pos) {
			return fmt.Errorf("invalid %q atom size %d", typ, size)
		}
		a := atom{typ: typ, payload: pos + int64(headerSize), end: pos + int64(size)}
		if err := visit(a); err != nil {
			return err
		}
		pos = a.end
	}
	return nil
}
