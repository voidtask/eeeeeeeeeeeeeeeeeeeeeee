#!/bin/bash

AV1_CRF=32
AV1_PRESET=4
AV1_PARAMS="keyint=10s:fast-decode=2"

# Directory containing the .mp4 files (current directory)
input_dir="./"
# Directory to move files smaller than 120MB
small_files_dir="$input_dir/LESS-THAN-120MB"
# Directory to move input files after they are processed
processed_inputs_dir="$input_dir/PROCESSED-INPUTS"
# Directory to keep AV1 versions
output_dir="$input_dir/OUTPUT-DIR"
# Directory to keep file that is currently being converted
temp_dir="$input_dir/.TEMP"

# Create the LESS-THAN-120MB and PROCESSED-INPUTS directories if they don't exist
mkdir -p "$small_files_dir"
mkdir -p "$processed_inputs_dir"
mkdir -p "$output_dir"
mkdir -p "$temp_dir"

handle_sigint() {
    echo "Script interrupted. Exiting..."
    exit 1
}

trap handle_sigint SIGINT

# Second iteration: Process remaining files with ffmpeg
echo "Processing files..."
for input_file in "$input_dir"/*.mp4; do
    # Get the base name of the file (without extension)
    base_name=$(basename "$input_file" .mp4)

    # Define the output file name
    output_file="$temp_dir/$base_name [AV1 10bit][2K].mkv"

    # Run the ffmpeg command
    taskset -c 0-7 ffmpeg -y -i "$input_file" -map 0:v -map 0:a -map 0:s? -vf "scale=-1:'min(1440,ih)'" -c:v libsvtav1 -svtav1-params $AV1_PARAMS -preset $AV1_PRESET -crf $AV1_CRF -pix_fmt yuv420p10le -c:a copy -c:s copy "$output_file"

    echo "Processed $input_file"

    # Move the processed input file to the PROCESSED-INPUTS directory
    mv "$input_file" "$processed_inputs_dir/"
    echo "Moved $input_file to $processed_inputs_dir"

    mv "$output_file" "$output_dir/"
    echo "Moved AV1 result file to output directory"
done

echo "All done!"
