{
  mkShell,
  callPackage,

  # extra tooling
  go,
  gopls,
  gofumpt,
  goreleaser,
}:
let
  defaultPackage = callPackage ./package.nix { };
in
mkShell {
  inputsFrom = [ defaultPackage ];

  packages = [
    go
    gopls
    gofumpt
    goreleaser
  ];
}
