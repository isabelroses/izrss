self: {
  config,
  lib,
  pkgs,
  ...
}:
with lib; {
  meta.maintainers = [maintainers.isabelroses];

  options.programs.izrss = {
    enable = mkEnableOption "A fast and once simple cli todo tool";

    urls = mkOption {
      type = with types; listOf str;
      example = ["http://example.com"];
      description = "Feed URLs.";
    };
  };

  config = let
    cfg = config.programs.izrss;
  in
    mkIf cfg.enable {
      home.packages = [self.packages.${pkgs.stdenv.hostPlatform.system}.default];

      xdg.configFile."izrss/urls" = mkIf (cfg.urls != []) {text = concatStringsSep "\n" cfg.urls;};
    };
}
