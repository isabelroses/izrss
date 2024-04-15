{
  lib,
  buildGoModule,
  version ? "unstable",
}:
buildGoModule {
  pname = "izrss";
  inherit version;

  src = ./.;

  vendorHash = "sha256-BCHxt1jSh6xYchGnRAITdwvNsRXfwVNiKmee5Tda0bQ=";

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
  ];

  meta = {
    description = "A RSS feed reader";
    homepage = "https://github.com/isabelroses/izrss";
    license = with lib.licenses; [gpl3];
    maintainers = with lib.maintainers; [isabelroses];
    platforms = lib.platforms.all;
  };
}
