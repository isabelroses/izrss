{
  go,
  gopls,
  gofumpt,
  hyperfine,
  goreleaser,
  callPackage,
}:
let
  mainPkg = callPackage ./default.nix { };
in
mainPkg.overrideAttrs (oa: {
  nativeBuildInputs = [
    go
    gopls
    gofumpt
    hyperfine # lets benchmark
    goreleaser
  ] ++ (oa.nativeBuildInputs or [ ]);
})
