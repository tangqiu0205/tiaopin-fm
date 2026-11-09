# Audio regression fixtures

These fixtures are synthetic tones generated for this project, not recorded music or broadcasts. Keep this provenance and the regeneration commands with the test data.

## AAC-LC

These are synthetic tones generated for this project with FFmpeg 9.0.2's native AAC encoder. They contain no recorded music or broadcasts. FFmpeg is only used to generate fixtures; running the tests and application does not require it.

- `lc-stereo-48000.aac`: 48 kHz stereo, left 997 Hz and right 1601 Hz, AAC-LC/ADTS at 96 kbps.
- `lc-mono-32000.aac`: 32 kHz mono, 997 Hz, AAC-LC/ADTS at 48 kbps.

Recreate from the project root (remove existing fixtures first, or use FFmpeg's overwrite option deliberately):

```bash
ffmpeg -f lavfi -i 'aevalsrc=0.2*sin(2*PI*997*t)|0.15*sin(2*PI*1601*t):s=48000:d=0.35' -c:a aac -profile:a aac_low -b:a 96k -f adts internal/player/testdata/aac/lc-stereo-48000.aac
ffmpeg -f lavfi -i 'sine=frequency=997:sample_rate=32000:duration=0.35' -ac 1 -c:a aac -profile:a aac_low -b:a 48k -f adts internal/player/testdata/aac/lc-mono-32000.aac
```

## Ogg

Synthetic 0.35-second tones, with no recorded music or broadcasts. Stereo
fixtures contain 997 Hz on the left and 1601 Hz on the right; mono contains
997 Hz. FFmpeg is only needed to regenerate them, not to run the application
or tests.

Opus's input rate is informational. The native mpv tests decode all fixtures
with null audio output and verify real frequency-band metadata.

Recreate from the project root:

```bash
ffmpeg -f lavfi -i 'aevalsrc=0.3*sin(2*PI*997*t)|0.3*sin(2*PI*1601*t):s=48000:d=0.35' -c:a libvorbis -q:a 4 internal/player/testdata/ogg/vorbis-stereo-48000.ogg
ffmpeg -f lavfi -i 'sine=frequency=997:sample_rate=32000:duration=0.35' -c:a libvorbis -q:a 4 internal/player/testdata/ogg/vorbis-mono-32000.ogg
ffmpeg -f lavfi -i 'aevalsrc=0.3*sin(2*PI*997*t)|0.3*sin(2*PI*1601*t):s=48000:d=0.35' -c:a libopus -b:a 96k internal/player/testdata/ogg/opus-stereo-48000.ogg
ffmpeg -f lavfi -i 'sine=frequency=997:sample_rate=24000:duration=0.35' -c:a libopus -b:a 48k internal/player/testdata/ogg/opus-mono-24000.ogg
```

Run `TestMPVFormatsAndSpectrum` with `TIAOPIN_TEST_MPV=1` and
`TIAOPIN_MPV_LIBRARY` pointing to the prepared native library. Prepare the runtime from the project root with
`python scripts/prepare_mpv.py`.
