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

func ffmpegProcess(input string, output string, preset int, crf int) {
	args := []string{
		"-c",
		"0-7",
		"ffmpeg",
		"-y",
		"-i", input,
		"-map", "0:v",
		"-map", "0:a",
		"-map", "0:s?",
		"-vf", "scale=-1:'min(1440,ih)'",
		"-c:v", "libsvtav1",
		"-svtav1-params", "keyint=10s:fast-decode=2",
		"-preset", strconv.Itoa(preset),
		"-crf", strconv.Itoa(crf),
		"-pix_fmt", "yuv420p10le",
		"-c:a", "copy",
		"-c:s", "copy",
		output,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, "taskset", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println(cmd.String())

	err := cmd.Run()

	switch {
	case ctx.Err() != nil:
		log.Fatal("Interrupted")
	case err != nil:
		log.Fatalf("ffmpeg error:\n%s\n", err)
	}
}

func inputFiles(inputDir string) []string {
	matches, err := filepath.Glob(filepath.Join(inputDir, "*.mp4"))
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
	crf := *flag.Int("crf", 32, "CRF value passed to SVT-AV1")
	preset := *flag.Int("preset", 4, "Preset value passed to SVT-AV1")
	maxHeight := *flag.Int("maxheight", 1440, "Maximum height of output video")

	inputDir := *flag.String("dir", "./", "Directory that will be used to scan for videos")
	doneDir := *flag.String("donedir", "./_processed", "Directory where processed files will be moved")
	outDir := *flag.String("outdir", "./_out", "Directory where processed files go to")
	tempDir := *flag.String("tempdir", "./_temp", "Directory where currently processed file output will be stored")

	makeDirs(doneDir, outDir, tempDir)

	inputs := inputFiles(inputDir)

	for _, path := range inputs {
		var displayHeight int

		srcRes, err := ffprobeResolution(path)
		if err != nil {
			displayHeight = maxHeight
		}

		displayHeight = min(maxHeight, srcRes[1])

		srcBase := filepath.Base(path)
		srcExt := filepath.Ext(path)

		newBase := strings.TrimSuffix(srcBase, srcExt)
		newBase += fmt.Sprintf(" (%dp) [AV1 10bit].mkv", displayHeight)

		tempPath := filepath.Join(tempDir, newBase)

		ffmpegProcess(path, tempPath, preset, crf)
	}

	fmt.Println("\n[AV1] Done!")
}
