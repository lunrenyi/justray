<p align="center">
  <img src=".github/assets/logotype.png" width="480" alt="justray">
</p>

<p align="center">
  <a href="https://github.com/luynrs/justray/commits/main"><img alt="Last commit" src="https://img.shields.io/github/last-commit/luynrs/justray?style=for-the-badge&logo=github&logoColor=white&labelColor=1e1e2e&color=cba6f7"></a>
  <img alt="Platforms" src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-cba6f7?style=for-the-badge&logo=linux&logoColor=white&labelColor=1e1e2e">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-GPL--3.0-cba6f7?style=for-the-badge&logo=gnu&logoColor=white&labelColor=1e1e2e"></a>
</p>

<p align="center">A modern VPN client that lives in your terminal</p>

<p align="center">
    <img src=".github/assets/cli.gif" width="100%" alt="justray showcase">
</p>

## Features

- **Modern protocols:** VMess, VLESS, Trojan, Shadowsocks, Hysteria1/2, TUIC, AnyTLS, SOCKS5 and more
- **Flexible:** subscriptions from raw links or Clash/Mihomo YAML, auto-refreshing and a wide list of settings
- **Lightweight:** up to ~50 MB of RAM on unix-based systems, ~100 MB on Windows
- **Crossplatform:** runs in every terminal on macOS, Linux and Windows (PowerShell and WSL)

## Install
> [!TIP]
> You can re-run the script at any time to update client

<details>
<summary>Windows</summary>

```powershell
irm https://raw.githubusercontent.com/luynrs/justray/main/install.ps1 | iex
```

</details>

<details>
<summary>Linux</summary>

Script:

```sh
curl -fsSL https://raw.githubusercontent.com/luynrs/justray/main/install.sh | sh
```

Nix / NixOS:

```sh
nix run github:luynrs/justray
```

```nix
# flake.nix
{ inputs.justray.url = "github:luynrs/justray"; }
```

```nix
# home-manager: justrayd as a systemd --user service
{
  imports = [ justray.homeManagerModules.default ];
  services.justray.enable = true;
}
```

</details>

<details>
<summary>macOS</summary>

Script:

```sh
curl -fsSL https://raw.githubusercontent.com/luynrs/justray/main/install.sh | sh
```

Nix:

```sh
nix run github:luynrs/justray
```

</details>

## Usage

If `justray` is installed via a script or package manager, the `jray` alias is available.

`jray`: open the TUI

#### Connection

- `jray up <node> [--tun | --proxy]`: start the daemon
- `jray down`: disconnect and shut down
- `jray status`: show connection status

#### Subscriptions

`jray subscription` or `jray sub`

- `add`: add a subscription or raw protocol link
- `remove`: remove a subscription or node by ID
- `list`: list subscriptions and nodes

#### Options

- `-h`, `--help`: show help
- `-v`, `--version`: show the current version

## License

[GPL-3.0](LICENSE)
