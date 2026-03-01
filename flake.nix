{
  description = "purge - bulk message deletion tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "purge";
          version = "0.0.0-dev";
          src = ./.;

          # Run `nix build` once, it will fail and print the correct hash.
          # Replace this with the real hash from that output.
          vendorHash = pkgs.lib.fakeHash;

          meta = with pkgs.lib; {
            description = "Bulk message deletion tool";
            homepage = "https://github.com/pavlo/purge";
            license = licenses.mit;
            mainProgram = "purge";
          };
        };
      }
    );
}
