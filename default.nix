{
  lib,
  buildGoModule,
}: let
  version = "0.0.3";
in
  buildGoModule {
    pname = "izrss";
    inherit version;

    src = ./.;

    vendorHash = "sha256-VzGkfB4x4fEqZCsil8hjUzlvNIl/8W7MRQd+7wRViU0=";

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
