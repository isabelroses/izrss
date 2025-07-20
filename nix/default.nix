{
  lib,
  rustPlatform,
  openssl,
  pkg-config,
  version ? "unstable",
}:
rustPlatform.buildRustPackage {
  pname = "izrss";
  inherit version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.intersection (lib.fileset.fromSource (lib.sources.cleanSource ../.)) (
      lib.fileset.unions [
        ../Cargo.toml
        ../Cargo.lock
        ../src
      ]
    );
  };

  buildInputs = [ openssl ];
  nativeBuildInputs = [ pkg-config ];

  cargoLock.lockFile = ../Cargo.lock;

  meta = {
    description = "A RSS feed reader for the terminal";
    homepage = "https://github.com/isabelroses/izrss";
    license = lib.licenses.gpl3Plus;
    maintainers = with lib.maintainers; [ isabelroses ];
    mainProgram = "izrss";
  };
}
