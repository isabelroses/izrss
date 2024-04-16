{
  lib,
  buildGoModule,
  version ? "unstable",
}:
buildGoModule {
  pname = "izrss";
  inherit version;

  src = ../.;

  vendorHash = "sha256-u50qWuZH2VjnHWjCMeEYFKsVxQarNbx7ixZ+aJ8xOFw=";

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
  ];

  meta = {
    description = "A RSS feed reader for the teminal";
    homepage = "https://github.com/isabelroses/izrss";
    license = lib.licenses.gpl3Only;
    maintainers = with lib.maintainers; [isabelroses];
    mainPackage = "izrss";
  };
}
