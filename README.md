<div align="center">
 <img src="./.github/assets/logo.svg" width="128" alt="izrss logo">
 <h1>izrss</h1>

 <p>An RSS feed reader for the terminal.</p>
</div>

&nbsp;

![demo](./.github/assets/demo.gif)

### Usage & Customization

The main bulk of customization is done via the `~/.config/izrss/config.toml` file. You can find an example file here [config.toml](./example.toml).

Then run `izrss` to read the feeds.

Feeds are on the left and the selected feed's posts on the right; "All posts"
at the top lists everything, newest first. Errors appear briefly as toasts in
the corner and are kept in `~/.local/state/izrss/izrss.log`.

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | move (scroll in the reader) |
| `J` `K` | jump 5 |
| `←` `→` / `h` `l` / `tab` | switch pane |
| `enter` | open the feed's posts, then the post |
| `/` | search every post |
| `x` / `X` | toggle read / mark the feed read |
| `r` / `R` | refresh the feed / all feeds |
| `o` | open the post in your browser |
| `?` | show all keys |
| `esc` / `q` | back; `q` on the main screen quits |

### Images

Posts show their images inline in terminals that support the kitty graphics
protocol's Unicode placeholders, such as [Ghostty](https://ghostty.org) and
[kitty](https://sw.kovidgoyal.net/kitty/). Elsewhere, and inside tmux or
screen, images appear as their alt text linking to the image.

Set `images = "on"` or `"off"` under `[reader]` to override the detection.

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
