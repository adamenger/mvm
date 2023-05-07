# mvm - music video maker
mvm is a little utility that will generate a video with an imperfect EQ of your incoming audio. 

Order of operations:
1. mvm takes a WAV file as input
2. Creates a spectrogram from wav data
3. Normalizes spectrogram
4. Then draws shapes for each frequency band to create a video with a unique EQ visualization
5. Renders shapes onto frames which are output in named order
6. Frames are sent to ffmpeg for rendering

# Installing
Clone repo and run `make`.

You will also need `ffmpeg` installed.

# Usage:
```
Usage of ./mvm:
  -bar-count int
        number of eq bars to render, more bars equals thinner bars overall. 32, 64, 128 are good places to start (default 64)
  -base-color string
        base color used to color the EQ bars, accepts a hex value (default "ffffff")
  -frame-dir string
        directory used for frame output (default "frames")
  -hpf int
        high pass filter, used to cut off lower frequencies which can overpower the bottom end of the visualization
  -input-file string
        The file to use for input to the music video maker (default "input.wav")
  -keep-frames
        whether or not to keep the frames output directory
  -output-file string
        file to write output to (default "output.mp4")
  -rainbow
        rainbow mode does exactly what it says, applies a gradient rainbow to the EQ
  -scale-factor float
        adjust this value to change the shape movement, you may want to up the hpf when upping scale-factor
  -smoothing-size float
        smoothing size, attempts to smooth out movement between frames (default 0.5)
  -window-size int
        sample window size (default 4096)
```

## Example Run
```
./mvm --frame-dir=/mnt/c/users/adame/downloads/frames --input-file input.wav --output-file /mnt/c/users/adame/Downloads/output4.mp4 -scale-factor 2.0 -hpf 30 -bar-count 32 --rainbow
2023/05/06 19:30:11 successfully read input.wav
2023/05/06 19:30:11 decoded sample input.wav
2023/05/06 19:30:11 applying high-pass filter
2023/05/06 19:30:11 creating spectrogram
2023/05/06 19:30:13 normalized spectrogram
2023/05/06 19:30:13 created output dir for frames /mnt/c/users/adame/downloads/frames
2023/05/06 19:30:13 generated hash
2023/05/06 19:30:13 rendering frames
[==================================================] 100% (6199/6199)
2023/05/06 19:31:46 sending frames to ffmpeg
ffmpeg version 4.2.7-0ubuntu0.1 Copyright (c) 2000-2022 the FFmpeg developers
  built with gcc 9 (Ubuntu 9.4.0-1ubuntu1~20.04.1)
...ffmpeg output
...ffmpeg output
...ffmpeg output
2023/05/06 19:33:32 cleaning up...removing frames directory
2023/05/06 19:33:48 frames deleted
2023/05/06 19:33:48 Your music video is ready at /mnt/c/users/adame/Downloads/output4.mp4
```

## Example Video 1

In this run, we have the following arguments set:

* --scale-factor 1.0
* --hpf 100
* --bar-count 8
* --smoothing-size 0.2
* --base-color f28500 # should be a tangerine orange

Audio is from [The Atomic Music Machine on Soundcloud](https://soundcloud.com/the-atomic-music-machine/goodbye-winter)

[![example video](https://i.ytimg.com/vi/BRkeC26M82o/hqdefault.jpg)](https://www.youtube.com/watch?v=BRkeC26M82o)

## Example Video 2

In this run, we have the following arguments set:
* --scale-factor 2.0
* --hpf 300
* --bar-count 32
* --base-color FF3659
* --smoothing-size 0.6

Audio is [Snowbridge by The Atomic Music Machine](https://soundcloud.com/the-atomic-music-machine/snowbridge)

[![example video](https://i.ytimg.com/vi/6O0GNW-yCn0/hqdefault.jpg)](https://www.youtube.com/watch?v=6O0GNW-yCn0)

## Example Video 3

In this run, we have the following arguments set:
* --scale-factor 5.0
* --hpf 30
* --bar-count 64
* --rainbow

[![example video](https://i.ytimg.com/vi/mg_OhM-pwA8/hqdefault.jpg)](https://www.youtube.com/watch?v=mg_OhM-pwA8)
