# Third-party notices

## Audio playback and sampling

TiaopinFM dynamically loads **libmpv** for playback, frequency analysis and
recognition sampling. It does not use Go MP3/AAC/Vorbis/Opus decoders or launch
an FFmpeg executable. libmpv itself uses FFmpeg and other native components.

Every platform uses the same pinned runtime from
[mpv-audio](https://github.com/tangqiu0205/mpv-audio), release
`libmpv-audio-0.42.0-r1` (mpv 0.42.0). `scripts/mpv.lock.json` records the
release, asset name and archive SHA-256 for each platform; the preparation
script verifies the archive, every bundled library, the platform, the mpv
version and the LGPL build profile before replacing anything.

- [mpv](https://github.com/mpv-player/mpv): built with `-Dgpl=false`; LGPL-2.1+
  for that configuration. See the bundled [Copyright](https://github.com/mpv-player/mpv/blob/b0841e8c4d156b8403dc1876c4cbd9011eb583f3/Copyright).
  The build applies one recorded patch that links an existing CoreAudio helper
  file; it changes no mpv licensing or behavior.
- [FFmpeg](https://ffmpeg.org/legal.html): built with `--disable-gpl
  --disable-nonfree`; LGPL-2.1+ for that configuration.
- [libass](https://github.com/libass/libass) and
  [libplacebo](https://github.com/haasn/libplacebo): ISC and LGPL-2.1+ as
  recorded in each bundle.
- Font, TLS and runtime support libraries come from the build environment of
  the mpv-audio pipeline. Their package versions, source metadata and license
  files are recorded under `licenses/packages/` in every bundle, and the bundle
  manifest marks them `supporting_licenses_review_required`.

The release is currently a **candidate** (`channel: candidate`), not a signed
or audited distribution. Bundled binaries are separate, replaceable files; they
are not embedded in the Go executable.

### Redistribution checklist

Before publishing binaries, the distributor must verify the exact native
build's license configuration, collect all required copyright/license notices,
and provide the corresponding source and build materials required by each
license. The mpv-audio release includes per-platform source archives for mpv,
FFmpeg, libass and libplacebo (with pinned submodules and applied patches), but
**not** the source of the supporting distribution libraries. A version tag for
mpv alone is not the complete corresponding source for the dynamically linked
dependencies. Generated package metadata aids this verification; it is not a
substitute for fulfilling source distribution obligations.

## Song recognition

**github.com/alexgorbatchev/goshazam v0.8.1** — MIT. This generates fingerprints
and calls the recognition service; it is not an audio playback/codec backend.
See [source](https://github.com/alexgorbatchev/goshazam/tree/v0.8.1) and the
included [MIT license](third_party/licenses/goshazam-MIT.txt).

## Scope

These notices cover selected native audio and recognition dependencies, not
all dependencies. Go module versions are pinned in `go.mod` and `go.sum`.
TiaopinFM's AGPL-3.0 license and original upstream attribution are documented
in [README.md](README.md) and [LICENSE](LICENSE).
