<div align="center">
 <h1>izrss</h1>

 <p>An RSS feed reader for the terminal.</p>
</div>

&nbsp;

![demo](./.github/assets/demo.gif)

### Usage & Customization

The main bulk of customization is done via the `~/.config/izrss/config.toml` file. You can find an example file here [config.toml](./example.toml).

The rest of the config is done via using the environment variables `GLAMOUR_STYLE`.
For a good example see: [catppuccin/glamour](https://github.com/catppuccin/glamour)

Then run `izrss` to read the feeds.

### Installation

<details>

<summary>

#### With Nix flakes and home-manager

</summary>

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    izrss.url = "github:isabelroses/izrss";
  };

  outputs = { self, nixpkgs, home-manager, izrss }: {
    homeConfigurations."user@hostname" = home-manager.lib.homeManagerConfiguration {
      modules = [
        home-manager.homeManagerModules.default
        izrss.homeManagerModules.default
        {
          programs.izrss = {
            enable = true;
            settings.urls = [
              "https://isabelroses.com/rss.xml"
              "https://uncenter.dev/feed.xml"
            ];
          };
        }
      ];
    };
  }
}
```

</details>

<details>

<summary>

#### With Nix flakes

</summary>

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    izrss.url = "github:isabelroses/izrss";
  };

  outputs = { self, nixpkgs, izrss }: {
    nixosConfigurations.example = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [{
        environment.systemPackages = [
          inputs.izrss.packages.${pkgs.system}.default
        ];
      }];
    };
  }
}
```

</details>
