self: {
  config,
  lib,
  pkgs,
  ...
}: let
  inherit (lib) mkIf types mkOption mkEnableOption concatStringsSep;
in {
  meta.maintainers = [lib.maintainers.isabelroses];

  options.programs.izrss = {
    enable = mkEnableOption "A fast and once simple cli todo tool";

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
        text = mkOption {
          type = types.str;
          example = "#cdd6f4";
          default = "";
          description = "Text color";
        };

        subtext = mkOption {
          type = types.str;
          example = "#a6adc8";
          default = "";
          description = "Subtext color";
        };

        inverttext = mkOption {
          type = types.str;
          example = "#1e1e2e";
          default = "";
          description = "Inverted text color";
        };

        accent = mkOption {
          type = types.str;
          example = "#74c7ec";
          default = "";
          description = "Accent color";
        };

        borders = mkOption {
          type = types.str;
          example = "#313244";
          default = "";
          description = "Borders color";
        };
      };
    };
  };

  config = let
    cfg = config.programs.izrss;
  in
    mkIf cfg.enable {
      home.packages = [self.packages.${pkgs.stdenv.hostPlatform.system}.default];

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
