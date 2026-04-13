{
  description = "A Go program for Claude Code's statusline that displays session info in a compact, Nerd Font-styled format with clickable OSC 8 hyperlinks.";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    go-overlay.url = "github:purpleclay/go-overlay";
    go-overlay.inputs.nixpkgs.follows = "nixpkgs";
    go-overlay.inputs.flake-utils.follows = "flake-utils";
  };

  outputs = {
    nixpkgs,
    flake-utils,
    go-overlay,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [go-overlay.overlays.default];
        };
        go = pkgs.go-bin.fromGoMod ./go.mod;
      in {
        packages.default = pkgs.buildGoApplication {
          pname = "cc-statusline";
          version = "1.2.0";
          src = ./.;
          inherit go;
          modules = ./govendor.toml;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            go.withDefaultTools
            go-overlay.packages.${system}.govendor
          ];
        };
      }
    );
}
