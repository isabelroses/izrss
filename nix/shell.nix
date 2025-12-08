{
  mkShellNoCC,
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
mkShellNoCC {
  inputsFrom = [ defaultPackage ];

  packages = [
    go
    gopls
    gofumpt
    goreleaser
  ];
}
