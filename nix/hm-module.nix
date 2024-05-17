self:
{
  lib,
  pkgs,
  config,
  ...
}:
let
  inherit (lib)
    mkIf
    types
    mkOption
    mkEnableOption
    mkPackageOption
    concatStringsSep
    ;

  settingsFormat = pkgs.formats.toml { };
in
{
  meta.maintainers = [ lib.maintainers.isabelroses ];

  options.programs.izrss = {
    enable = mkEnableOption "A fast and once simple cli todo tool";

    package = mkPackageOption self.packages.${pkgs.stdenv.hostPlatform.system} "izrss" { };

    urls = mkOption {
      type = with types; listOf str;
      example = [ "http://example.com" ];
      description = "Feed URLs.";
    };

    settings = mkOption {
      inherit (settingsFormat) type;
      default = { };
      example = lib.literalExpression ''
        dateformat = "02/01/2006"

        [colors]
        text = "#cdd6f4"
        inverttext = "#1e1e2e"
        subtext = "#a6adc8"
        accent = "#74c7ec"
        borders = "#313244"
      '';
      description = ''
        Configuration written to {file}`$XDG_CONFIG_HOME/izrss/config.toml`.

        See <https://github.com/isabelroses/izrss/blob/main/example.toml> for the documentation.
      '';
    };
  };

  config =
    let
      cfg = config.programs.izrss;
    in
    mkIf cfg.enable {
      home.packages = [ cfg.package ];

      xdg.configFile = {
        "izrss/urls" = mkIf (cfg.urls != [ ]) { text = concatStringsSep "\n" cfg.urls; };
        "izrss/config.toml".source = mkIf (cfg.settings != { }) settingsFormat.generate "izrss-config.toml" cfg.settings;
      };

      assertions = [
        {
          assertion = config.xdg.enable;
          message = "Option xdg.enable must be enabled for the configuration to be written to the filesystem.";
        }
      ];
    };
}
