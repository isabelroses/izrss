{
  lib,
  buildGoModule,
  version ? "unstable",
}:
buildGoModule {
  pname = "izrss";
  inherit version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.intersection (lib.fileset.fromSource (lib.sources.cleanSource ../.)) (
      lib.fileset.unions [
        ../go.mod
        ../go.sum
        ../main.go
        ../internal
      ]
    );
  };

  vendorHash = "sha256-hiqheaGCtybrK5DZYz2GsYvTlUZDGu04wDjQqfE7O3k=";

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  meta = {
    description = "A RSS feed reader for the terminal";
    homepage = "https://github.com/isabelroses/izrss";
    license = lib.licenses.gpl3Plus;
    maintainers = with lib.maintainers; [ isabelroses ];
    mainProgram = "izrss";
  };
}
