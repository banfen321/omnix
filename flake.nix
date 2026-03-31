{
  description = "omnix - AI-Powered Nix Dev Environment Generator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "omnix";
          version = "0.1.0";
          src = ./.;
          
          # Use vendor directory to avoid needing to calculate vendorHash
          vendorHash = null;

          postInstall = ''
            mv $out/bin/omnix $out/bin/omnix
          '';

          meta = with pkgs.lib; {
            description = "Zero-config AI generator for Nix flake environments";
            homepage = "https://github.com/banfen321/omnix";
            license = licenses.mit;
            mainProgram = "omnix";
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls ];
        };
      }
    );
}
