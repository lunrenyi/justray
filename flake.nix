
# Usage:
#   nix run .#justxray               # try it without installing (jray is a short alias)
#   nix build .#justxray              # build ./result/bin/{justxray,jray,justxrayd}
#
# home-manager (flake-based):
#   {
#     imports = [ justxray.homeManagerModules.default ];
#     services.justxray.enable = true;    # runs justxrayd as a systemd --user service
#   }
{
  description = "justxray - lightweight terminal vpn client built on xray-core";

  inputs = {
    nixpkgs.url = "git+https://github.com/NixOS/nixpkgs.git?ref=nixos-unstable&shallow=1";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;

      justxrayFor = system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in pkgs.buildGoModule {
          pname = "justxray";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-NH8fJYf0d4FAXnhFnS/E5gtfbwchAXEcNj4K0RDHGJE=";

          subPackages = [ "cmd/justxray" "cmd/justxrayd" ];
          ldflags = [ "-s" "-w" ];
          nativeBuildInputs = [ pkgs.makeWrapper ];

          postInstall = ''
            ln -s justxray $out/bin/jray
            for bin in justxray justxrayd; do
              wrapProgram $out/bin/$bin --prefix PATH : ${pkgs.xray}/bin
            done
          '';

          meta = with pkgs.lib; {
            description = "TUI VPN subscription client for xray-core, with a background daemon that owns the xray-core process so the TUI can exit without dropping the connection";
            homepage = "https://github.com/luynrs/justxray";
            mainProgram = "justxray";
            platforms = platforms.unix;
          };
        };
    in
    {
      packages = forAllSystems (system: {
        justxray = justxrayFor system;
        default = justxrayFor system;
      });

      apps = forAllSystems (system: {
        justxray = { type = "app"; program = "${justxrayFor system}/bin/justxray"; };
        jray = { type = "app"; program = "${justxrayFor system}/bin/jray"; };
        justxrayd = { type = "app"; program = "${justxrayFor system}/bin/justxrayd"; };
        default = { type = "app"; program = "${justxrayFor system}/bin/justxray"; };
      });

      devShells = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = pkgs.mkShell {
            packages = [ pkgs.go pkgs.gopls pkgs.xray pkgs.golangci-lint ];
          };
        }
      );

      homeManagerModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.justxray;
          system = pkgs.stdenv.hostPlatform.system;
        in
        {
          options.services.justxray = {
            enable = lib.mkEnableOption "the justxray background daemon (justxrayd), as a systemd --user service";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${system}.justxray;
              defaultText = lib.literalExpression "justxray.packages.<system>.default";
              description = "The justxray package providing the justxray/jray TUI and the justxrayd daemon.";
            };

            xrayPackage = lib.mkOption {
              type = lib.types.package;
              default = pkgs.xray;
              defaultText = lib.literalExpression "pkgs.xray";
              description = "The xray-core package justxrayd should exec as its managed process.";
            };
          };

          config = lib.mkIf cfg.enable {
            home.packages = [ cfg.package ];

            systemd.user.services.justxrayd = {
              Unit = {
                Description = "justxray background daemon (xray-core subscription manager)";
                After = [ "network-online.target" ];
                Wants = [ "network-online.target" ];
              };
              Service = {
                ExecStart = "${cfg.package}/bin/justxrayd --xray-bin ${cfg.xrayPackage}/bin/xray";
                Restart = "on-failure";
                RestartSec = 2;
              };
              Install.WantedBy = [ "default.target" ];
            };
          };
        };
      homeManagerModules.justxray = self.homeManagerModules.default;
    };
}
