{
  description = "izrss - A RSS reader for the terminal";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "x86_64-darwin"
        "i686-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems =
        function: nixpkgs.lib.genAttrs systems (system: function nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: rec {
        default = izrss;
        izrss = pkgs.callPackage ./nix/default.nix { version = self.shortRev or "unstable"; };
      });

      overlays.default = final: _: {
        izrss = final.callPackage ./nix/default.nix { version = self.shortRev or "unstable"; };
      };

      devShells = forAllSystems (pkgs: {
        default = pkgs.callPackage ./nix/shell.nix { };
      });

      homeManagerModules.default = import ./nix/hm-module.nix self;
    };
}
