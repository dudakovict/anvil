#!/usr/bin/env bash
# Renames the module and service after cloning the starter:
#   scripts/rename.sh github.com/you/my-service [service-name]
# service-name defaults to the last path segment of the module.
set -euo pipefail

NEW_MODULE=${1:?usage: scripts/rename.sh <new-module-path> [service-name]}
OLD_MODULE=$(go list -m)
OLD_SERVICE=$(basename "$OLD_MODULE")
SERVICE=${2:-$(basename "$NEW_MODULE")}

git grep -l "$OLD_MODULE" -- ':!docs' ':!scripts' | xargs -r sed -i "s|$OLD_MODULE|$NEW_MODULE|g"
go mod edit -module "$NEW_MODULE"

git grep -l "$OLD_SERVICE" -- ':!docs' ':!scripts' | xargs -r sed -i "s/$OLD_SERVICE/$SERVICE/g"

go mod tidy
go tool swag init -g cmd/api/main.go -o docs

echo "renamed: $OLD_MODULE -> $NEW_MODULE (service: $SERVICE)"

read -r -p "reset git history to a fresh initial commit? [y/N] " ans
if [ "${ans:-N}" = y ]; then
	rm -rf .git
	git init -q
	git add -A
	git commit -q -m "Initial commit"
	echo "git history reset"
fi
