# mvm - music video maker
mvm is a little utility that will generate a video with an imperfect EQ of your incoming audio.

# Installing
Clone repo and run `make`

# Usage:
```
Usage of ./mvm:
  -frame-dir string
        directory used for frame output (default "frames")
  -input-file string
        The file to use for input to the music video maker (default "input.wav")
  -keep-frames
        whether or not to keep the frames output directory, default: false
  -output-file string
        file to write output to (default "output.mp4")
  -window-size int
        window size (default 4096)
```

## Example Run
```
./mvm --frame-dir=/mnt/c/users/adame/downloads/frames --input-file input4.wav --output-file /mnt/c/users/adame/Downloads/output.mp4
2023/05/06 09:39:39 successfully read input4.wav
2023/05/06 09:39:39 creating spectrogram
2023/05/06 09:39:41 normalized spectrogram
2023/05/06 09:39:41 generated hash
2023/05/06 09:39:41 rendering frames
[==================================================] 100% (3100/3100)
2023/05/06 09:40:08 sending frames to ffmpeg
ffmpeg version 4.2.7-0ubuntu0.1 Copyright (c) 2000-2022 the FFmpeg developers
....ffmpeg output
....ffmpeg output
2023/05/06 09:40:44 cleaning up...removed frames directory
2023/05/06 09:40:44 your music video is ready!!: /mnt/c/users/adame/Downloads/output.mp4
```

The video below is the result which is in `output.mp4` in this example.

## Example Video

Click the screenshot below to view video on YouTube

[![example video](https://i.ytimg.com/vi/mg_OhM-pwA8/hqdefault.jpg)](https://www.youtube.com/watch?v=mg_OhM-pwA8)
