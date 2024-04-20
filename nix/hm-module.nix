self: {
  config,
  lib,
  pkgs,
  ...
}: let
  inherit (lib) mkIf types mkOption mkEnableOption mkPackageOption concatStringsSep;

  mkColorOption = name: color:
    mkOption {
      type = types.str;
      example = color;
      default = "";
      description = "${name} color";
    };
in {
  meta.maintainers = [lib.maintainers.isabelroses];

  options.programs.izrss = {
    enable = mkEnableOption "A fast and once simple cli todo tool";

    package = mkPackageOption self.packages.${pkgs.stdenv.hostPlatform.system} "izrss" {};

    urls = mkOption {
      type = with types; listOf str;
      example = ["http://example.com"];
      description = "Feed URLs.";
    };

    settings = {
      dateformat = mkOption {
        type = types.str;
        example = "02/01/2006";
        default = "";
        description = ''
          The date format show for when a post was published
          see <https://go.dev/src/time/format.go> for more information
        '';
      };

      colors = {
        text = mkColorOption "Text" "#cdd6f4";
        subtext = mkColorOption "Subtext" "#a6adc8";
        inverttext = mkColorOption "Invert text" "#1e1e2e";
        accent = mkColorOption "Accent" "#74c7ec";
        borders = mkColorOption "Borders" "#313244";
      };
    };
  };

  config = let
    cfg = config.programs.izrss;
  in
    mkIf cfg.enable {
      home.packages = [cfg.package];

      xdg.configFile = {
        "izrss/urls" = mkIf (cfg.urls != []) {text = concatStringsSep "\n" cfg.urls;};
        "izrss/config.toml".source = (pkgs.formats.toml {}).generate "config.toml" cfg.settings;
      };

      assertions = [
        {
          assertion = config.xdg.enable;
          message = "Option xdg.enable must be enabled for the configuration to be written to the filesystem.";
        }
      ];
    };
}
