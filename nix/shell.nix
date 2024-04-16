{
  go,
  gopls,
  hyperfine,
  goreleaser,
  callPackage,
}: let
  mainPkg = callPackage ./default.nix {};
in
  mainPkg.overrideAttrs (oa: {
    nativeBuildInputs =
      [
        go
        gopls
        hyperfine # lets benchmark
        goreleaser
      ]
      ++ (oa.nativeBuildInputs or []);
  })
