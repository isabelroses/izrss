{
  description = "izrss - A RSS reader for the terminal";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = {
    self,
    nixpkgs,
    ...
  }: let
    systems = ["x86_64-linux" "x86_64-darwin" "i686-linux" "aarch64-linux" "aarch64-darwin"];
    forAllSystems = nixpkgs.lib.genAttrs systems;
    pkgsForEach = nixpkgs.legacyPackages;
  in {
    packages = forAllSystems (system: rec {
      izrss = pkgsForEach.${system}.callPackage ./nix/default.nix {
        version = self.shortRev or "unstable";
      };

      default = izrss;
    });

    devShells = forAllSystems (system: {
      default = pkgsForEach.${system}.callPackage ./nix/shell.nix {};
    });

    homeManagerModules.default = import ./nix/hm-module.nix self;
  };
}
