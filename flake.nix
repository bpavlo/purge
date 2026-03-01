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

          vendorHash = "sha256-5wdT0k7JLR9zkB/nTXSW7IL8piaW8hLQYp1LLHBVa6c=";

          meta = with pkgs.lib; {
            description = "Bulk message deletion tool";
            homepage = "https://github.com/bpavlo/purge";
            license = licenses.mit;
            mainProgram = "purge";
          };
        };
      }
    );
}
