# WSL2 ARM64 CGO Cross-Compile (Agent Guide)

How to build **Raspberry Pi / Linux ARM64** binaries of TARR Annunciator **with CGO** from this Windows machine, using **Debian in WSL2**.

This is the supported path for Pi releases that need `faiface/beep` / `hajimehoshi/oto` (ALSA). Plain `GOOS=linux GOARCH=arm64 go build` on Windows **without** a Linux ARM64 C toolchain will not produce a working audio binary.

---

## Current machine status (as of setup)

| Item | Value |
|------|--------|
| Distro | Debian 13 (trixie), **WSL2** |
| WSL user | `ariellenbogen` |
| Go | **1.27.1** at `~/.local/go` (user-local; preferred over apt `/usr/bin/go`) |
| Cross GCC | `aarch64-linux-gnu-gcc` 14.2 |
| ALSA for target | `libasound2-dev:arm64` |
| Env helpers | `~/.profile_tarr_cross` (sourced from `~/.bashrc`) |
| First successful test binary | `releases/tarr-annunciator-raspberry-pi-arm64-wsl-test` |

Repo on Windows is visible inside WSL as:

```text
/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator
```

**Space in `Ari Ellenbogen` breaks poorly quoted commands.** Prefer scripts under paths without spaces, or always pass the full path as a **single** argument.

---

## What agents can do autonomously

Cursor agents on this Windows host **can** drive WSL2 Debian via:

```powershell
wsl -d Debian -- <command...>
```

Examples that work well:

```powershell
wsl -d Debian -- uname -r
wsl -d Debian -- bash "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_build_pi_arm64.sh"
```

### Hard requirements / pitfalls

1. **Do not nest complex bash in PowerShell `-c` strings** with `$HOME`, `$PATH`, pipes, or `$(...)`. PowerShell expands or mangles them. **Write a `.sh` file and run that file.**
2. **Paths with spaces:** pass the script path as one argv element. If launching via Windows Terminal `wt.exe`, prefer copying the script into WSL home first:
   ```text
   /home/ariellenbogen/.local/bin/...
   ```
3. **`sudo` needs a password.** Non-interactive `sudo -n` fails. For apt installs, launch a visible `wsl.exe` console (not a broken restarted WT tab) so the user can type their Debian password once. Prefer scripts already under `~/.local/bin/` for that.
4. **WSL is for compile/link, not Pi audio QA.** Do not treat WSL `alsamixer` / WSLg sound as equivalent to Raspberry Pi ALSA device selection.

---

## One-command Pi CGO build (preferred)

From Windows PowerShell / agent Shell tool:

```powershell
wsl -d Debian -- bash "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_build_pi_arm64.sh"
```

What it does:

1. Puts `~/.local/go/bin` first on `PATH`
2. Loads `~/.profile_tarr_cross` and calls `tarr_arm64_env`
3. Builds `source/` with `CGO_ENABLED=1 GOOS=linux GOARCH=arm64`
4. Writes:
   - WSL copy: `~/.local/share/tarr/tarr-annunciator-raspberry-pi-arm64`
   - Windows copy: `releases/tarr-annunciator-raspberry-pi-arm64-wsl-test`

Expect ~20MB ELF `ARM aarch64`, dynamically linked.

Runtime libs on the **Pi** (not needed for compiling here):

- `libasound.so.2`
- `libc.so.6`

Those are normal on Raspberry Pi OS. Missing them is a **run** problem on the device, not a failed cross-compile.

---

## Manual build (inside Debian)

```bash
export PATH="$HOME/.local/go/bin:$PATH"
source "$HOME/.profile_tarr_cross"
tarr_arm64_env

cd "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/source"
go build -o "$HOME/.local/share/tarr/tarr-annunciator-raspberry-pi-arm64" .
file "$HOME/.local/share/tarr/tarr-annunciator-raspberry-pi-arm64"
```

Or:

```bash
source ~/.profile_tarr_cross
cd "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/source"
tarr_arm64_build "$HOME/.local/share/tarr/tarr-annunciator-raspberry-pi-arm64"
```

### Environment set by `tarr_arm64_env`

```bash
CGO_ENABLED=1
GOOS=linux
GOARCH=arm64
CC=aarch64-linux-gnu-gcc
CXX=aarch64-linux-gnu-g++
PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig
PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig
CGO_CFLAGS="-I/usr/aarch64-linux-gnu/include -I/usr/include/aarch64-linux-gnu"
CGO_LDFLAGS="-L/usr/aarch64-linux-gnu/lib -L/usr/lib/aarch64-linux-gnu -lasound"
```

---

## Verify toolchain

```powershell
wsl -d Debian -- bash "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_verify_arm64_toolchain.sh"
```

Success looks like:

- `go version go1.27.x linux/amd64` from `~/.local/go`
- `aarch64-linux-gnu-gcc` present
- `pkg-config` finds ALSA under the arm64 libdir
- Smoke C programs compile/link as `ELF 64-bit ... ARM aarch64` (including `#include <alsa/asoundlib.h>`)
- Prints `TOOLCHAIN_OK`

Status / packages check:

```powershell
wsl -d Debian -- bash "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_check_arm64_status.sh"
```

Ready marker file (written after package install):

```text
~/.local/share/tarr/arm64_cross_ready
```

---

## Helper scripts in this repo

| Script | Purpose |
|--------|---------|
| `docs/agent_shared/wsl_setup_arm64_cross.sh` | Install/update user-local Go + (with sudo) packages; writes `~/.profile_tarr_cross`. Use `--go-only` to skip apt. |
| `docs/agent_shared/wsl_install_arm64_packages.sh` | Apt-only half (needs interactive sudo). |
| `docs/agent_shared/wsl_build_pi_arm64.sh` | **Main agent entry:** CGO linux/arm64 build of `source/`. |
| `docs/agent_shared/wsl_verify_arm64_toolchain.sh` | Compiler / ALSA link smoke tests. |
| `docs/agent_shared/wsl_check_arm64_status.sh` | Package + ready-marker status. |

Copy into WSL home when launching interactive sudo windows (avoids space-in-path breakage):

```text
/home/ariellenbogen/.local/bin/wsl_install_arm64_packages.sh
```

---

## Reinstall / repair

### Update Go only (no sudo)

```powershell
wsl -d Debian -- bash "/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_setup_arm64_cross.sh" --go-only
```

### Reinstall cross packages (needs user password)

1. Copy installer into WSL home (no spaces).
2. Open a **fresh** console with plain `wsl.exe` (do **not** â€œrestartâ€ an old Windows Terminal tab that still has a broken `/mnt/c/Users/Ari` command):

```powershell
wsl -d Debian -- bash -c "cp '/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_install_arm64_packages.sh' `$HOME/.local/bin/ && chmod +x `$HOME/.local/bin/wsl_install_arm64_packages.sh"
Start-Process -FilePath "wsl.exe" -ArgumentList @("-d","Debian","--","bash","/home/ariellenbogen/.local/bin/wsl_install_arm64_packages.sh")
```

Packages that must be present:

- `build-essential`, `pkg-config`, `file`
- `gcc-aarch64-linux-gnu`, `g++-aarch64-linux-gnu`
- `libc6-dev-arm64-cross`, `linux-libc-dev-arm64-cross`
- `libasound2-dev:arm64` (requires `dpkg --add-architecture arm64`)

Confirm WSL2 (not WSL1):

```powershell
wsl -l -v
wsl -d Debian -- uname -r
```

Kernel should contain `microsoft-standard-WSL2`. Distro **VERSION** column should be `2`.

---

## What success / failure means

| Result | Meaning |
|--------|---------|
| `BUILD_OK` + `file` shows `ARM aarch64` | Cross-compile worked. Ship candidate for Pi runtime test. |
| Missing `aarch64-linux-gnu-gcc` / ALSA headers | Toolchain incomplete; run package install. |
| Binary needs `libasound.so.2` at runtime | **Expected.** Install/run on Pi; not a compile failure on WSL. |
| Audio weird in WSL | Irrelevant for release gating. Test audio on a real Pi. |

---

## Quick agent checklist before a Pi release build

1. `wsl -l -v` â†’ Debian is version **2**
2. `wsl_verify_arm64_toolchain.sh` â†’ `TOOLCHAIN_OK`
3. `wsl_build_pi_arm64.sh` â†’ `BUILD_OK`
4. Confirm artifact is `ELF ... ARM aarch64` and copy/deploy to Pi for smoke run
5. Do **not** claim audio-device QA from WSL alone
