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

    vendorHash = "sha256-UFP9F4zNpA+FP3OFMm5OZbqA8hKGOz0d966wQJN5gK4=";

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
