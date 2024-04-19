{
  lib,
  buildGoModule,
  version ? "unstable",
}:
buildGoModule {
  pname = "izrss";
  inherit version;

  src = ../.;

  vendorHash = "sha256-gH5AFroreBD0tQmT99Bmo2pAdPkiPWUNGsmKX4p3/JA=";

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  meta = {
    description = "A RSS feed reader for the teminal";
    homepage = "https://github.com/isabelroses/izrss";
    license = lib.licenses.gpl3Only;
    maintainers = with lib.maintainers; [isabelroses];
    mainPackage = "izrss";
  };
}
