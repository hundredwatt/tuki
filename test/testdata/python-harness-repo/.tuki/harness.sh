#!/usr/bin/env sh

if ! command -v python3 > /dev/null 2>&1; then
  echo "python3 is not installed. Exiting."
  exit 1
fi

python3