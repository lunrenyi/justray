
# Usage:
#   nix run .#justray               # try it without installing (jray is a short alias)
#   nix build .#justray              # build ./result/bin/{justray,jray,justrayd}
#
# home-manager (flake-based):
#   {
#     imports = [ justray.homeManagerModules.default ];
#     services.justray.enable = true;    # runs justrayd as a systemd --user service
#   }
{
  description = "A lightweight terminal-based VPN client";

  inputs = {
    nixpkgs.url = "git+https://github.com/NixOS/nixpkgs.git?ref=nixos-unstable&shallow=1";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;

      justrayFor = system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in pkgs.buildGoModule {
          pname = "justray";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-x2jAAVAaPu8MPCkE7fhPkXbatwlQBVsh9cPH58IL3Do=";

          subPackages = [ "cmd/justray" "cmd/justrayd" ];
          tags = [ "with_quic" "with_utls" "with_gvisor" ];
          ldflags = [ "-s" "-w" ];

          postInstall = ''
            ln -s justray $out/bin/jray
          '';

          meta = with pkgs.lib; {
            description = "A lightweight terminal-based VPN client";
            homepage = "https://github.com/luynrs/justray";
            license = licenses.gpl3Plus;
            mainProgram = "justray";
            platforms = platforms.unix;
          };
        };
    in
    {
      packages = forAllSystems (system: {
        justray = justrayFor system;
        default = justrayFor system;
      });

      apps = forAllSystems (system: {
        justray = { type = "app"; program = "${justrayFor system}/bin/justray"; };
        jray = { type = "app"; program = "${justrayFor system}/bin/jray"; };
        justrayd = { type = "app"; program = "${justrayFor system}/bin/justrayd"; };
        default = {
          type = "app";
          program = "${justrayFor system}/bin/justray";
          meta.description = "A lightweight terminal-based VPN client";
        };
      });

      devShells = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = pkgs.mkShell {
            packages = [ pkgs.go pkgs.gopls pkgs.golangci-lint ];
          };
        }
      );

      homeManagerModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.justray;
          system = pkgs.stdenv.hostPlatform.system;
        in
        {
          options.services.justray = {
            enable = lib.mkEnableOption "the justray background daemon (justrayd), as a systemd --user service";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${system}.justray;
              defaultText = lib.literalExpression "justray.packages.<system>.default";
              description = "The justray package providing the justray/jray TUI and the justrayd daemon.";
            };
          };

          config = lib.mkIf cfg.enable {
            home.packages = [ cfg.package ];

            systemd.user.services.justrayd = {
              Unit = {
                Description = "justray background daemon";
                After = [ "network-online.target" ];
                Wants = [ "network-online.target" ];
              };
              Service = {
                ExecStart = "${cfg.package}/bin/justrayd";
                Restart = "on-failure";
                RestartSec = 3;
                AmbientCapabilities = [ "CAP_NET_ADMIN" ];
              };
              Install.WantedBy = [ "default.target" ];
            };
          };
        };
      homeManagerModules.justray = self.homeManagerModules.default;
    };
}
