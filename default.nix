{
  lib,
  buildGoModule,
}:
buildGoModule {
  pname = "izrss";
  version = "0.0.1";

  src = ./.;

  vendorHash = "sha256-a/Zo5ISGYhIrYR1i2UGd2dBqA7Qn3EW9MemzTtm0HNA";

  ldflags = ["-s" "-w"];

  meta = {
    description = "A RSS feed reader";
    homepage = "https://github.com/isabelroses/izrss";
    license = with lib.licenses; [gpl3];
    maintainers = with lib.maintainers; [isabelroses];
    platforms = lib.platforms.all;
  };
}
