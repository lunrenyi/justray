<p align="center">
  <img src=".github/assets/logotype.png" width="480" alt="justray">
</p>

<p align="center">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-GPL--3.0-cba6f7?style=for-the-badge&logo=gnu&logoColor=white&labelColor=1e1e2e"></a>
  <img alt="Platforms" src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-cba6f7?style=for-the-badge&labelColor=1e1e2e">
  <img alt="Go" src="https://img.shields.io/badge/go-1.26-cba6f7?style=for-the-badge&logo=go&logoColor=white&labelColor=1e1e2e">
</p>

<p align="center">A fast, lightweight, and modern VPN subscription client with a terminal UI.</p>

## Features

- VMess, VLess, Trojan, Shadowsocks, Hysteria, Hysteria2, TUIC, AnyTLS, SOCKS5, WireGuard
- Subscriptions from raw links or Clash/Mihomo YAML, auto-refreshing
- Crossplatform, eats only ~50mb of ram in TUN-mode

## Install

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

```sh
justray              # open the TUI
justray up [name]    # connect (last-used node if omitted)
justray down         # disconnect
justray status       # show connection status
justray sub add <url>    # add a subscription
justray sub list         # list subscriptions and nodes
justray sub remove <id>  # remove a subscription
```

## License

[GPL-3.0](LICENSE)
