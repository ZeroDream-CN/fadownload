# FA Download

A small CLI tool that downloads Font Awesome CSS and webfonts from the official asset CDN.

> **For education, learning, and testing only.**  
> Before using Pro fonts in a commercial project, purchase a license from [Font Awesome Plans](https://fontawesome.com/plans). This tool does **not** grant any Font Awesome license and does not waive compliance with their terms.

## Features

- Automatically download CSS and webfonts from fontawesome website
- Support downloading a specific version
- Support a custom output directory
- Show download progress in real time

## Requirements

- Go 1.22+ (run or build locally)
- [UPX](https://upx.github.io/) for release builds via `build.sh`

## Usage

```bash
# Latest version → ./download/releases/vX.X.X/
./fadownload

# Specific version
./fadownload -v 7.3.1

# Custom output (no download/releases/vX.X.X/ prefix)
./fadownload -o ./fa
# → ./fa/css/...
# → ./fa/webfonts/...

# Combined
./fadownload -v 6.5.2 -o ./assets/fontawesome

# Help
./fadownload -h
```

| Flag | Alias | Description |
|------|-------|-------------|
| `-version` | `-v` | Version to download, e.g. `v7.3.1` or `7.3.1` (default: latest from the site) |
| `-output` | `-o` | Output directory; writes `css/` and `webfonts/` directly |
| `-no-color` | | Disable colored output |

## Build

```bash
./build.sh
```

Artifacts under `dist/`:

- `fadownload-linux-amd64`
- `fadownload-windows-amd64.exe`

## License

This project is released under the [MIT License](LICENSE).

## Disclaimer

This project is intended for education and testing only. Do not use it for unauthorized commercial purposes. Font Awesome and its Pro icons/fonts remain the property of their respective owners. For commercial use, buy a suitable plan at [fontawesome.com/plans](https://fontawesome.com/plans) and follow the official license terms.
