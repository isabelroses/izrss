{
  lib,
  buildGoModule,
  version ? "unstable",
}:
buildGoModule {
  pname = "izrss";
  inherit version;

  src = ../.;

  vendorHash = "sha256-/gD82wT4jWNMQsGPb2nYQiFQsUdKICyO6eiRPHrLsy8=";

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
    mainPackage = "izrss";
  };
}
