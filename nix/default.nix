{
  lib,
  buildGoModule,
  version ? "unstable",
}:
buildGoModule {
  pname = "izrss";
  inherit version;

  src = ../.;

  vendorHash = "sha256-Vu5Zi12iFmh/PYRZ1QvFpSwikQQB4zI+DHk/u6A0CdI=";

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  meta = {
    description = "A RSS feed reader for the terminal";
    homepage = "https://github.com/isabelroses/izrss";
    license = lib.licenses.gpl3Only;
    maintainers = with lib.maintainers; [isabelroses];
    mainPackage = "izrss";
  };
}
