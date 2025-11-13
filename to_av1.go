package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	crf          int
	preset       int
	maxHeight    int
	noMaxHeight  bool
	svtAv1Params string
)

var (
	inputPattern string
	inputDir     string
	doneDir      string
	outDir       string
	tempDir      string
)

func init() {
	flag.IntVar(&crf, "crf", 32, "CRF value passed to SVT-AV1")
	flag.IntVar(&preset, "preset", 4, "Preset value passed to SVT-AV1")
	flag.IntVar(&maxHeight, "maxheight", 1440, "Maximum height of output video")
	flag.BoolVar(&noMaxHeight, "nomaxheight", false, "Disalbes maximum height filter, -maxheight flag will be ignored if set to true")
	flag.StringVar(&svtAv1Params, "svtav1-params", "keyint=10s:fast-decode=2", "SVT-AV1 params passed to ffmpeg command")

	flag.StringVar(&inputPattern, "pattern", "*.mp4", "Input video files pattern")
	flag.StringVar(&inputDir, "dir", "./", "Directory that will be used to scan for videos")
	flag.StringVar(&doneDir, "processeddir", "./_processed", "Directory where processed files will be moved")
	flag.StringVar(&outDir, "outdir", "./_out", "Directory where processed files go to")
	flag.StringVar(&tempDir, "tempdir", "./_temp", "Directory where currently processed file output will be stored")
}

func makeDirs(paths ...string) {
	for _, path := range paths {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func ffprobeResolution(path string) ([]int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return []int{-1, -1}, err
	}

	lines := strings.Split(string(out), "\n")

	result := []int{}
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		num, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			num = -1
		}
		result = append(result, num)
	}

	return result, nil
}

func ffmpegProcess(input string, output string) {
	args := []string{
		"-c",
		"0-7",
		"ffmpeg",
		"-y",
		"-i", input,
		"-map", "0:v",
		"-map", "0:a",
		"-map", "0:s?",
	}

	if !noMaxHeight {
		args = append(args, "-vf", fmt.Sprintf("scale=-1:'min(%d,ih)'", maxHeight))
	}

	args = append(args,
		"-c:v", "libsvtav1",
		"-svtav1-params", svtAv1Params,
		"-preset", strconv.Itoa(preset),
		"-crf", strconv.Itoa(crf),
		"-pix_fmt", "yuv420p10le",
		"-c:a", "copy",
		"-c:s", "copy",
		output,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, "taskset", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println("----------------")
	fmt.Println(cmd.String())
	fmt.Println()

	err := cmd.Run()

	switch {
	case ctx.Err() != nil:
		log.Fatal("Interrupted")
	case err != nil:
		log.Fatalf("ffmpeg error:\n%s\n", err)
	}
}

func inputFiles(inputDir string) []string {
	matches, err := filepath.Glob(filepath.Join(inputDir, inputPattern))
	if err != nil {
		panic(err)
	}

	absPaths := []string{}
	for _, m := range matches {
		abs, err := filepath.Abs(m)
		if err == nil {
			absPaths = append(absPaths, abs)
		}
	}

	return absPaths
}

func main() {
	flag.Parse()
	makeDirs(doneDir, outDir, tempDir)

	for _, inpPath := range inputFiles(inputDir) {
		var displayHeight int

		srcRes, err := ffprobeResolution(inpPath)

		switch {
		case err != nil && !noMaxHeight:
			displayHeight = maxHeight
		case err == nil && !noMaxHeight:
			displayHeight = min(maxHeight, srcRes[1])
		case err == nil && noMaxHeight:
			displayHeight = srcRes[1]
		default:
			displayHeight = -1
		}

		inpBase := filepath.Base(inpPath)
		inpExt := filepath.Ext(inpPath)

		newBase := strings.TrimSuffix(inpBase, inpExt)
		if !noMaxHeight {
			newBase += fmt.Sprintf(" (%dp)", displayHeight)
		}
		newBase += " [AV1 10bit].mkv"

		tempPath := filepath.Join(tempDir, newBase)

		ffmpegProcess(inpPath, tempPath)

		tempPathAbs, err := filepath.Abs(tempPath)
		if err != nil {
			continue
		}

		outPathAbs, err := filepath.Abs(filepath.Join(outDir, newBase))
		if err != nil {
			continue
		}

		if err := os.Rename(tempPathAbs, outPathAbs); err != nil {
			continue
		}

		donePathAbs, err := filepath.Abs(filepath.Join(doneDir, inpBase))
		if err != nil {
			continue
		}

		if err := os.Rename(inpPath, donePathAbs); err != nil {
			continue
		}
	}

	fmt.Println("\n[AV1] Done!")
}
